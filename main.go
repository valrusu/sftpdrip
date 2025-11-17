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
	stopNoFiles := flag.Int("stopnofiles", 0, "stop when no files available to pull after this many tries")

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
		logged := false
		noFiles := 0

		for {
			files, err := client.ReadDir(*dir)
			if err != nil {
				log.Panicf("failed to read dir: %v", err)
			}
			// log.Println("got list of files")

			if len(files) == 0 && len(prevfiles) == 0 {
				if !logged {
					log.Println("🕐 waiting for files")
					logged = true
				}
				noFiles++
				if *stopNoFiles > 0 && noFiles >= *stopNoFiles {
					log.Println("no new files for a long time")
					break
				}
			}

			// files with same size can be downloaded
			for _, f := range files {
				for _, pf := range prevfiles {
					if f.Name() == pf.Name() {
						if f.Size() == pf.Size() {
							log.Println("🔄 download and delete", f.Name())
							logged = false
							noFiles = 0

							remoteFilePath := (*dir) + "/" + f.Name()
							localFilePath := f.Name()

							// Open remote file for reading
							remoteFile, err := client.Open(remoteFilePath)
							if err != nil {
								log.Fatalf("failed to open remote file: %s, %v", remoteFilePath, err)
							}

							// Create local file for writing
							localFile, err := os.Create(localFilePath)
							if err != nil {
								log.Fatalf("failed to create local file: %v", err)
							}

							// Copy from remote to local
							bytesCopied, err := io.Copy(localFile, remoteFile)
							if err != nil {
								log.Fatalf("failed to copy file: %v", err)
							}

							log.Printf("✅ downloaded %s (%d bytes)\n", localFilePath, bytesCopied)

							if err := client.Remove(remoteFilePath); err != nil {
								log.Panicf("Failed to remove remote file %s %v", remoteFilePath, err)
							}
							log.Printf("✅ removed %s\n", remoteFilePath)

							remoteFile.Close()
							localFile.Close()
						} else {
							if !logged {
								// log.Println("file changed", f.Name(), f.Size(), pf.Size())
								log.Println("uploading file detected", f.Name())
								logged = true
							}
						}
					}
				}
			}
			// if true || !logged {
			// log.Println("🕐 prev", len(prevfiles), "files, crt", len(files), "files")
			// logged = true
			// }
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

		log.Println("files to load:", filesToLoad)

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
					log.Println("🕐 waiting for download of", len(files), "files", files[0].Name())
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
