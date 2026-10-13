package cloudstore

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"google.golang.org/api/googleapi"
	"io"
)

type GCS struct{ Bucket *storage.BucketHandle }

func (g *GCS) Load(ctx context.Context, key string) ([]byte, int64, error) {
	reader, err := g.Bucket.Object("workspaces/" + key + ".db").NewReader(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()
	if reader.Attrs.Size > MaxSnapshot {
		return nil, 0, errors.New("workspace exceeds snapshot limit")
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxSnapshot+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > MaxSnapshot {
		return nil, 0, errors.New("workspace exceeds snapshot limit")
	}
	return data, reader.Attrs.Generation, nil
}
func (g *GCS) Save(ctx context.Context, key string, data []byte, generation int64) error {
	conditions := storage.Conditions{GenerationMatch: generation}
	if generation == 0 {
		conditions = storage.Conditions{DoesNotExist: true}
	}
	writer := g.Bucket.Object("workspaces/" + key + ".db").If(conditions).NewWriter(ctx)
	writer.ContentType = "application/octet-stream"
	writer.CacheControl = "no-store"
	writer.ChunkSize = 1 << 20
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return translate(err)
	}
	return translate(writer.Close())
}
func translate(err error) error {
	var api *googleapi.Error
	if errors.As(err, &api) && api.Code == 412 {
		return ErrConflict
	}
	return err
}
