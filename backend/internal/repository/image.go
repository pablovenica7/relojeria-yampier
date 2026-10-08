package repository

import (
	"context"
	"errors"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"

	"relojeria-yampier/internal/domain"
)

// ImageBucket es el bucket GridFS donde se guardan las imágenes subidas
// desde el Admin (colecciones uploads.files y uploads.chunks). Guardarlas en
// la base, y no en el disco del contenedor, hace que sobrevivan a cada
// redeploy y que entren en los backups de MongoDB.
const ImageBucket = "uploads"

type ImageRepository struct{ db *mongo.Database }

func NewImageRepository(db *mongo.Database) *ImageRepository { return &ImageRepository{db: db} }

func (r *ImageRepository) bucket() (*gridfs.Bucket, error) {
	return gridfs.NewBucket(r.db, options.GridFSBucket().SetName(ImageBucket))
}

// Save guarda la imagen con el nombre dado (generado por el servidor).
func (r *ImageRepository) Save(ctx context.Context, filename, contentType string, content io.Reader) error {
	b, err := r.bucket()
	if err != nil {
		return err
	}
	stream, err := b.OpenUploadStream(filename,
		options.GridFSUpload().SetMetadata(bson.M{"contentType": contentType}))
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetWriteDeadline(deadline)
	}
	if _, err := io.Copy(stream, content); err != nil {
		_ = stream.Abort()
		return err
	}
	return stream.Close()
}

// Open abre la imagen más reciente con ese nombre. domain.ErrNotFound si no existe.
func (r *ImageRepository) Open(ctx context.Context, filename string) (*domain.Image, error) {
	var file struct {
		ID         interface{} `bson:"_id"`
		Length     int64       `bson:"length"`
		UploadDate time.Time   `bson:"uploadDate"`
		Metadata   struct {
			ContentType string `bson:"contentType"`
		} `bson:"metadata"`
	}
	err := r.db.Collection(ImageBucket+".files").FindOne(ctx, bson.M{"filename": filename},
		options.FindOne().SetSort(bson.D{{Key: "uploadDate", Value: -1}})).Decode(&file)
	if err != nil {
		return nil, mapErr(err)
	}
	b, err := r.bucket()
	if err != nil {
		return nil, err
	}
	stream, err := b.OpenDownloadStream(file.ID)
	if err != nil {
		if errors.Is(err, gridfs.ErrFileNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetReadDeadline(deadline)
	}
	return &domain.Image{Content: stream, Size: file.Length, ContentType: file.Metadata.ContentType, UploadedAt: file.UploadDate}, nil
}

// Exists indica si ya hay una imagen con ese nombre.
func (r *ImageRepository) Exists(ctx context.Context, filename string) (bool, error) {
	n, err := r.db.Collection(ImageBucket+".files").CountDocuments(ctx, bson.M{"filename": filename},
		options.Count().SetLimit(1))
	return n > 0, err
}
