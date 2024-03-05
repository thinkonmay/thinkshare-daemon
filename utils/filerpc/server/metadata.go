package server

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc/metadata"
)

// MD struct contains information about file being received
type MD struct {
	fileName string
	fileSize int64
}

// expandMetaData retrieves the metadata about the file being received from the stream context
func expandMetaData(ctx context.Context) (*MD, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("Failed to retrieve incoming context")
	}

	fileName := md.Get("file_name")

	if len(fileName) <= 0 {
		return nil, fmt.Errorf("Failed to retrieve incoming file name")
	}

	fileSizeAsString := md.Get("file_size")

	if len(fileSizeAsString) <= 0 {
		return nil, fmt.Errorf("Failed to retrieve incoming file size")
	}

	fileSize, err := strconv.ParseInt(fileSizeAsString[0], 10, 64)
	if err != nil || fileSize <= 0 {
		return nil, fmt.Errorf("received invalid file size")
	}

	return &MD{
		fileName: fileName[0],
		fileSize: fileSize,
	}, nil
}
