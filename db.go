package main

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CodeDoc is a document in the 'codes' collection: a Spotify auth code
// awaiting exchange when the user taps the deep link.
type CodeDoc struct {
	ID             bson.ObjectID `bson:"_id"`
	AuthCode       string        `bson:"authCode"`
	TelegramUserID string        `bson:"telegram_user_id"`
}

// UserDoc is a document in the 'users' collection (Spotify users).
type UserDoc struct {
	Username string `bson:"username"`
	Token    string `bson:"token"`
	Style    string `bson:"style"`
	TgID     string `bson:"tg_id"`
}

// LastFMUserDoc is a document in the 'lastfm' collection.
type LastFMUserDoc struct {
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
	client *mongo.Client
	codes  *mongo.Collection
	users  *mongo.Collection
	lastfm *mongo.Collection
}

func newDatabase(cfg *Config) (*Database, error) {
	uri := fmt.Sprintf(
		"mongodb://%s:%s@%s-shard-00-00.ibx7n.mongodb.net:27017,%s-shard-00-01.ibx7n.mongodb.net:27017,%s-shard-00-02.ibx7n.mongodb.net:27017/%s?ssl=true&replicaSet=atlas-lnda18-shard-0&authSource=admin&retryWrites=true&w=majority",
		cfg.MongoUser, cfg.MongoPass,
		cfg.MongoColl, cfg.MongoColl, cfg.MongoColl, cfg.MongoColl,
	)

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	db := client.Database(cfg.MongoColl) // db name mirrors the uri path segment
	return &Database{
		client: client,
		codes:  db.Collection("codes"),
		users:  db.Collection("users"),
		lastfm: db.Collection("lastfm"),
	}, nil
}

func (d *Database) Close() {
	// Leave time for Sentry to flush inside the container's 30s stop window.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.client.Disconnect(ctx); err != nil {
		logger.Error("mongo disconnect failed", "err", err)
	}
}

// -------- 'codes' collection --------

func (d *Database) AddCode(ctx context.Context, authCode, telegramUserID string) (*mongo.InsertOneResult, error) {
	return d.codes.InsertOne(ctx, bson.M{
		"authCode":         authCode,
		"telegram_user_id": telegramUserID,
	})
}

func (d *Database) FetchCode(ctx context.Context, id bson.ObjectID) (*CodeDoc, error) {
	var doc CodeDoc
	err := d.codes.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func (d *Database) DeleteCode(ctx context.Context, id bson.ObjectID) error {
	_, err := d.codes.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// -------- 'users' collection --------

func (d *Database) AddUser(ctx context.Context, tgID, token string) (*mongo.InsertOneResult, error) {
	return d.users.InsertOne(ctx, bson.M{
		"username": "User",
		"token":    token,
		"style":    "blur",
		"tg_id":    tgID,
	})
}

func (d *Database) FetchData(ctx context.Context, tgID string) (*UserDoc, error) {
	var doc UserDoc
	err := d.users.FindOne(ctx, bson.M{"tg_id": tgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
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

func (d *Database) AggregateUsers(ctx context.Context) ([]CountDoc, error) {
	pipeline := bson.A{
		bson.M{"$unwind": "$style"},
		bson.M{"$group": bson.M{"_id": "$style", "count": bson.M{"$sum": 1}}},
	}
	return d.aggregate(ctx, d.users, pipeline)
}

func (d *Database) AggregateLastFMUsers(ctx context.Context) ([]CountDoc, error) {
	pipeline := bson.A{
		bson.M{"$unwind": "$counter"},
		bson.M{"$group": bson.M{"_id": "$counter", "count": bson.M{"$sum": 1}}},
	}
	return d.aggregate(ctx, d.lastfm, pipeline)
}

func (d *Database) aggregate(ctx context.Context, coll *mongo.Collection, pipeline any) ([]CountDoc, error) {
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []CountDoc
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// -------- 'lastfm' collection --------

func (d *Database) AddLastFMUser(ctx context.Context, tgID, lastfmUsername string) (*mongo.InsertOneResult, error) {
	return d.lastfm.InsertOne(ctx, bson.M{
		"fm_username": lastfmUsername,
		"name":        "User",
		"tg_id":       tgID,
		"counter":     "on",
	})
}

func (d *Database) GetLastFMUser(ctx context.Context, tgID string) (*LastFMUserDoc, error) {
	var doc LastFMUserDoc
	err := d.lastfm.FindOne(ctx, bson.M{"tg_id": tgID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
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
