package steambackup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)



func ZipFolder(zipFileName, sourceFolder string) error {
	zipFile, err := os.Create(zipFileName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	err = filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceFolder, path)
		if err != nil {
			return err
		}

		zipPath := filepath.ToSlash(relPath)

		if info.IsDir() {
			zipPath += "/"
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_,err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return err
	})

	return err
}

func UnzipFolder(zipFileName, extractFolder string) error {
	zipReader, err := zip.OpenReader(zipFileName)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	err = os.MkdirAll(extractFolder, 0755)
	if err != nil {
		return err
	}

	for _, file := range zipReader.File {
		err := extractFile(file, extractFolder)
		if err != nil {
			return err
		}
	}

	return nil
}

func extractFile(file *zip.File, extractFolder string) error {
	zippedFile, err := file.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	extractPath := filepath.Join(extractFolder, file.Name)
	if file.FileInfo().IsDir() {
		os.MkdirAll(extractPath, file.Mode())
	} else {
		os.MkdirAll(filepath.Dir(extractPath), file.Mode())

		extractFile, err := os.Create(extractPath)
		if err != nil {
			return err
		}
		defer extractFile.Close()

		_, err = io.Copy(extractFile, zippedFile)
		if err != nil {
			return err
		}
	}

	return nil
}



func CopyFromBackupToTemp(source, destination string) {
	err := UnzipFolder(source+"/Backup.zip", destination)
	if err != nil {
		fmt.Printf("error backup folder : %s\n",err.Error())
	}
}

func CopyFromTempToBackup(source, destination string) {
	err := ZipFolder(destination+"/Backup.zip", source)
	if err != nil {
		fmt.Printf("error backup folder : %s\n",err.Error())
	}
}

var (
	stop = false
)

func StartBackup(source, backup string) {
	fmt.Println("start backing up folders")
	CopyFromBackupToTemp(backup, source)

	go func() {
		for {
			if stop {
				break
			}

			CopyFromTempToBackup(source, backup)
			time.Sleep(10 * time.Second)
		}
	}()
}

func StopBackup() {
	stop = true
}