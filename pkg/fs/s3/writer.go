package s3

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type writer struct {
	context  context.Context
	tempFile *os.File
	client   *s3.Client
	bucket   string
	key      string
}

func newWriter(context context.Context, s3Client *s3.Client, bucket string, key string) (*writer, error) {
	if tempFile, err := os.CreateTemp("", "rainsftp"); err != nil {
		return nil, err
	} else {
		return &writer{
			context:  context,
			tempFile: tempFile,
			client:   s3Client,
			bucket:   bucket,
			key:      key,
		}, nil
	}
}

func (writer *writer) WriteAt(buffer []byte, offset int64) (int, error) {
	return writer.tempFile.WriteAt(buffer, offset)
}

func (writer *writer) Close() error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(writer.bucket),
		Key:    aws.String(writer.key),
		Body:   writer.tempFile,
	}
	if _, err := writer.client.PutObject(writer.context, input); err != nil {
		writer.cleanUp()
		return err
	}

	return writer.cleanUp()
}

func (writer *writer) cleanUp() error {
	name := writer.tempFile.Name()

	// We can ignore the result of this operation as long as os.Remove succeeds.
	// We do not care if the data was successfully commit to the filesystem.
	writer.tempFile.Close()
	return os.Remove(name)
}
