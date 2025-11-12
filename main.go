package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func main() {

	clientType := flag.String("type", "", "push or pull")
	sftpServer := flag.String("sftpserver", "", "sftp server")
	sftpUser := flag.String("sftpuser", "", "sftp user")
	sftpPwd := flag.String("sftppwd", "", "sftp pwd")
	dir := flag.String("dir", "", "sftp dir")
	sleepPush := flag.Int("sleeppush", 10, "seconds to sleep on push side")
	sleepPull := flag.Int("sleeppull", 10, "seconds to sleep on pull side")

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println(" © Valentin Rusu 2005 / V-Systems")
	}
	flag.Parse()

	// SFTP connection configuration
	config := &ssh.ClientConfig{
		User: *sftpUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(*sftpPwd),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", *sftpServer, config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	log.Println("connected")

	client, err := sftp.NewClient(conn,
		sftp.UseConcurrentWrites(true),        // pipeline writes
		sftp.UseConcurrentReads(true),         // pipeline reads
		sftp.MaxConcurrentRequestsPerFile(64), // 64 outstanding requests)
	)
	if err != nil {
		log.Fatalf("Failed to create SFTP client: %v", err)
	}
	defer client.Close()
	log.Println("client created")

	if *clientType == "" {
		log.Panicln("need client type push or pull")
	}

	if *clientType == "pull" {

		var prevfiles []os.FileInfo

		for {
			files, err := client.ReadDir(*dir)
			if err != nil {
				log.Panicf("Failed to read dir: %v", err)
			}
			// log.Println("got list of files")

			// files with same size can be downloaded
		search:
			for _, f := range files {
				for _, pf := range prevfiles {
					if f.Name() == pf.Name() {
						if f.Size() == pf.Size() {
							log.Println("🔄 download and delete", f.Name())

							remoteFilePath := (*dir) + "/" + f.Name()
							localFilePath := f.Name()

							// Open remote file for reading
							remoteFile, err := client.Open(remoteFilePath)
							if err != nil {
								log.Fatalf("Failed to open remote file: %s, %v", remoteFilePath, err)
							}

							// Create local file for writing
							localFile, err := os.Create(localFilePath)
							if err != nil {
								log.Fatalf("Failed to create local file: %v", err)
							}

							// Copy from remote to local
							bytesCopied, err := io.Copy(localFile, remoteFile)
							if err != nil {
								log.Fatalf("Failed to copy file: %v", err)
							}

							fmt.Printf("✅ downloaded %s (%d bytes)\n", localFilePath, bytesCopied)

							client.Remove(remoteFilePath)
							fmt.Printf("✅ removed %s\n", remoteFilePath)

							remoteFile.Close()
							localFile.Close()

							break search
						} else {
							log.Println(f.Name(), f.Size(), pf.Size())
						}
					}
				}
			}
			log.Println("🕐 prev", len(prevfiles), "files, crt", len(files), "files")
			prevfiles = files
			time.Sleep(time.Duration(*sleepPull) * time.Second)
		}
	}

	if *clientType == "push" {
		// for each file
		//		wait for the dir to be empty ( by the pull side )
		//		load file
		//		wait until the file disappears (deleted by the pull client)

		filesToLoad := flag.Args()

		fmt.Println("files to load:", filesToLoad)

		// to be able to start the client while another file is being downloaded, I am not uploading unless the target dir is empty
		for _, fileName := range filesToLoad {

			// wait until dir is empty
			logged := false
			for {
				files, err := client.ReadDir(*dir)
				if err != nil {
					log.Panicf("Failed to read dir: %v", err)
				}

				if len(files) == 0 {
					break
				}

				if !logged {
					log.Println("🕐 waiting for download of", len(files), "files", files[0])
					logged = true
				}
				time.Sleep(time.Duration(*sleepPush) * time.Second)
			}

			// upload a file

			log.Println("🔄 uploading", fileName)

			srcFile, err := os.Open(fileName)
			if err != nil {
				log.Panicf("open local file: %s %w", fileName, err)
			}

			remoteFileName := (*dir) + "/" + fileName
			dstFile, err := client.Create(remoteFileName)
			if err != nil {
				log.Panicf("create remote file: %s %v", remoteFileName, err)
			}

			bytes, err := io.Copy(dstFile, srcFile)
			// bytes, err := io.CopyBuffer(dstFile, srcFile, copyBuffer)
			if err != nil {
				log.Panicf("copy data: %s %v", remoteFileName, err)
			}

			log.Printf("wrote %s %d\n", remoteFileName, bytes)

			dstFile.Close()
			srcFile.Close()
		}
	}
}
