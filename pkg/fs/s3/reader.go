package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type reader struct {
	object *s3.GetObjectOutput
	client *s3.Client
	key    string
	bucket string
}

func (reader reader) ReadAt(buffer []byte, offset int64) (int, error) {
	// Ensure the requested range falls within the bounds of the object's content length
	if offset < 0 || offset >= *reader.object.ContentLength {
		return 0, io.EOF
	}

	// Calculate the range of bytes to read based on the offset and buffer size
	rangeStart := offset
	rangeEnd := offset + int64(len(buffer)) - 1 // -1 to get inclusive end

	// Ensure rangeEnd does not exceed the object's content length
	if rangeEnd >= *reader.object.ContentLength {
		rangeEnd = *reader.object.ContentLength - 1
	}

	// Prepare the input parameters for the GetObject API call
	input := &s3.GetObjectInput{
		Key:    aws.String(reader.key),
		Bucket: aws.String(reader.bucket),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd)),
	}

	// Send request to S3 to get the specified range of bytes
	resp, err := reader.client.GetObject(context.Background(), input)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read the response body into the buffer
	bytesRead, err := io.ReadFull(resp.Body, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return bytesRead, err
	}

	return bytesRead, nil
}
