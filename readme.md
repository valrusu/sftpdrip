## For those times when you want to transfer a big amount of data through and SFTP server, but the space on the server is limited.
Instead of manually loading a file, wait, download it, delete it and repeat, start the sftpdrip on the push side, then start the sftpdrip on the pull side and come back to check later.
I used this all the time to transfer big amounts of data from a production database server to a development database server through an SFTP server with a limit of 5 GB. After spending one night doing this manually (split the source file, start uploading a 5 GB chunk, wait, start downloading the 5 GB chunk, wait, delete it and repeat) I wrote this utility to just be able to start it, go away and check the next day.
Hope it helps.

# Compile:

<pre>cd sftpdrip 
go build</pre>

# Example:

If needed, combine and split the input files to match the SFTP space. In this case I had many dmp files of various sizes:
Create a list of files named filePrefix.tar.gz.aa, filePrefix.tar.gz.ab etc, each with 4.8 GB in size (so I don't have surprises with the space limit on the SFTP server):

<pre> tar cv *.dmp | gzip -c | split -b 4800M - filePrefix.tar.gz. </pre>

Start the "pusher" side:
  
<pre> ./sftp_client -type push -sftpserver sftpserver:22 -sftpuser vvvv1513 -sftppwd xxxx1513 -dir /ftp/upload filePrefix.tar.gz.* </pre>

Start the "puller" side:

<pre> ./sftpdrip -type pull -sftpserver sftpserver:22 -sftpuser vvvv1513 -sftppwd xxxx1513 -dir /ftp/upload </pre>


# TODO
- Accept the sftp password from the keyboard or from env. The SFTP server and user are temporary for me so this was not a priority.

