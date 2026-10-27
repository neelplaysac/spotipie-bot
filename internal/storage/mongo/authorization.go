package mongostore

import (
	"context"
	"time"

	"github.com/neelplaysac/spotipie-go/internal/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// codeDocument is a completed session awaiting exchange through a Telegram link.
type codeDocument struct {
	ID             bson.ObjectID `bson:"_id"`
	AuthCode       string        `bson:"authCode"`
	TelegramUserID string        `bson:"telegram_user_id"`
}

func (d *Database) ensureAuthorizationIndexes(ctx context.Context) error {
	_, err := d.authorizations.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "state_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (d *Database) CreateAuthorization(ctx context.Context, stateHash, telegramID string, expiresAt time.Time) error {
	_, err := d.authorizations.InsertOne(ctx, bson.M{
		"state_hash": stateHash, "telegram_user_id": telegramID, "expires_at": expiresAt,
	})
	return err
}

func (d *Database) CompleteAuthorization(ctx context.Context, stateHash, code string, now time.Time) (string, error) {
	var doc codeDocument
	err := d.authorizations.FindOneAndUpdate(ctx, bson.M{
		"state_hash": stateHash, "expires_at": bson.M{"$gt": now},
		"authCode": bson.M{"$exists": false},
	}, bson.M{"$set": bson.M{"authCode": code}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return doc.ID.Hex(), nil
}

func (d *Database) ConsumeAuthorization(ctx context.Context, id, telegramID string, now time.Time) (*service.Code, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc codeDocument
	// Expiry is enforced here even before MongoDB's asynchronous TTL cleanup.
	err = d.authorizations.FindOneAndDelete(ctx, bson.M{
		"_id": oid, "telegram_user_id": telegramID,
		"expires_at": bson.M{"$gt": now}, "authCode": bson.M{"$exists": true},
	}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &service.Code{AuthCode: doc.AuthCode, TelegramUserID: doc.TelegramUserID}, nil
}
