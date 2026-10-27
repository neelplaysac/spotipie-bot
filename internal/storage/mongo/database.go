package mongostore

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

// userDocument is a document in the 'users' collection (Spotify users).
type userDocument struct {
	Username string `bson:"username"`
	Token    string `bson:"token"`
	Style    string `bson:"style"`
	TgID     string `bson:"tg_id"`
}

// lastFMDocument is a document in the 'lastfm' collection.
type lastFMDocument struct {
	FMUsername string `bson:"fm_username"`
	Name       string `bson:"name"`
	TgID       string `bson:"tg_id"`
	Counter    string `bson:"counter"`
}

type CountDoc struct {
	ID    string `bson:"_id"`
	Count int64  `bson:"count"`
}

type Database struct {
	logger         *slog.Logger
	client         *mongo.Client
	authorizations *mongo.Collection
	users          *mongo.Collection
	lastfm         *mongo.Collection
}

func New(cfg *config.Config, logger *slog.Logger) (*Database, error) {
	uri := cfg.MongoURI
	if uri == "" {
		uri = fmt.Sprintf(
			"mongodb://%s:%s@%s-shard-00-00.ibx7n.mongodb.net:27017,%s-shard-00-01.ibx7n.mongodb.net:27017,%s-shard-00-02.ibx7n.mongodb.net:27017/%s?ssl=true&replicaSet=atlas-lnda18-shard-0&authSource=admin&retryWrites=true&w=majority",
			url.QueryEscape(cfg.MongoUser), url.QueryEscape(cfg.MongoPass),
			cfg.MongoColl, cfg.MongoColl, cfg.MongoColl, cfg.MongoColl,
		)
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(10 * time.Second))
	if err != nil {
		// URI validation errors may echo credentials from the connection string.
		return nil, fmt.Errorf("invalid MongoDB connection configuration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = client.Disconnect(closeCtx)
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	db := client.Database(cfg.DatabaseName())
	store := &Database{
		logger: logger, client: client,
		authorizations: db.Collection("oauth_sessions"),
		users:          db.Collection("users"), lastfm: db.Collection("lastfm"),
	}
	if err := store.ensureAuthorizationIndexes(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("create authorization indexes: %w", err)
	}
	return store, nil
}

func (d *Database) Ping(ctx context.Context) error {
	return d.client.Ping(ctx, nil)
}

func (d *Database) Close() {
	// Leave time for Sentry to flush inside the container's 30s stop window.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.client.Disconnect(ctx); err != nil {
		d.logger.Error("mongo disconnect failed", "err", err)
	}
}

// -------- 'users' collection --------

func (d *Database) AddUser(ctx context.Context, tgID, token string) error {
	_, err := d.users.InsertOne(ctx, bson.M{
		"username": "User",
		"token":    token,
		"style":    "blur",
		"tg_id":    tgID,
	})
	return err
}

func (d *Database) FetchData(ctx context.Context, tgID string) (*service.User, error) {
	var doc userDocument
	err := d.users.FindOne(ctx, bson.M{"tg_id": tgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &service.User{Username: doc.Username, Token: doc.Token, Style: doc.Style, TgID: doc.TgID}, nil
}

func (d *Database) UpdateData(ctx context.Context, tgID, value string) error {
	_, err := d.users.UpdateOne(ctx, bson.M{"tg_id": tgID}, bson.M{"$set": bson.M{"username": value}})
	return err
}

func (d *Database) UpdateStyle(ctx context.Context, tgID, value string) error {
	_, err := d.users.UpdateOne(ctx, bson.M{"tg_id": tgID}, bson.M{"$set": bson.M{"style": value}})
	return err
}

func (d *Database) DeleteData(ctx context.Context, tgID string) error {
	_, err := d.users.DeleteOne(ctx, bson.M{"tg_id": tgID})
	return err
}

func (d *Database) CountAll(ctx context.Context) (int64, error) {
	return d.users.CountDocuments(ctx, bson.M{})
}

func (d *Database) AggregateUsers(ctx context.Context) ([]service.Count, error) {
	pipeline := bson.A{
		bson.M{"$unwind": "$style"},
		bson.M{"$group": bson.M{"_id": "$style", "count": bson.M{"$sum": 1}}},
	}
	return d.aggregate(ctx, d.users, pipeline)
}

func (d *Database) AggregateLastFMUsers(ctx context.Context) ([]service.Count, error) {
	pipeline := bson.A{
		bson.M{"$unwind": "$counter"},
		bson.M{"$group": bson.M{"_id": "$counter", "count": bson.M{"$sum": 1}}},
	}
	return d.aggregate(ctx, d.lastfm, pipeline)
}

func (d *Database) aggregate(ctx context.Context, coll *mongo.Collection, pipeline any) ([]service.Count, error) {
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []CountDoc
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	result := make([]service.Count, 0, len(out))
	for _, count := range out {
		result = append(result, service.Count{ID: count.ID, Count: count.Count})
	}
	return result, nil
}

// -------- 'lastfm' collection --------

func (d *Database) AddLastFMUser(ctx context.Context, tgID, lastfmUsername string) error {
	_, err := d.lastfm.InsertOne(ctx, bson.M{
		"fm_username": lastfmUsername,
		"name":        "User",
		"tg_id":       tgID,
		"counter":     "on",
	})
	return err
}

func (d *Database) GetLastFMUser(ctx context.Context, tgID string) (*service.LastFMUser, error) {
	var doc lastFMDocument
	err := d.lastfm.FindOne(ctx, bson.M{"tg_id": tgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &service.LastFMUser{FMUsername: doc.FMUsername, Name: doc.Name, TgID: doc.TgID, Counter: doc.Counter}, nil
}

func (d *Database) UpdateLastFMData(ctx context.Context, tgID, value string) error {
	_, err := d.lastfm.UpdateOne(ctx, bson.M{"tg_id": tgID}, bson.M{"$set": bson.M{"name": value}})
	return err
}

func (d *Database) RemoveLastFMUser(ctx context.Context, tgID string) error {
	_, err := d.lastfm.DeleteOne(ctx, bson.M{"tg_id": tgID})
	return err
}

func (d *Database) CountAllLastFM(ctx context.Context) (int64, error) {
	return d.lastfm.CountDocuments(ctx, bson.M{})
}

func (d *Database) ToggleCounter(ctx context.Context, tgID, value string) error {
	_, err := d.lastfm.UpdateOne(ctx, bson.M{"tg_id": tgID}, bson.M{"$set": bson.M{"counter": value}})
	return err
}

var _ service.AccountRepository = (*Database)(nil)
var _ service.AuthorizationRepository = (*Database)(nil)
var _ service.PlaybackRepository = (*Database)(nil)
