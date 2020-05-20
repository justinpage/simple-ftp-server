### Simple FTP server

> A concurrent File Transfer Protocol (FTP) server

```
Remote system type is UNIX.
Using binary mode to transfer files.
ftp> ls
227 Entering Passive Mode (127,0,0,1,215,236)
150 Accepted data connection
-rw-r--r--    1 justinpage  staff            10 May  20 13:28 message.md
drwxr-xr-x    3 justinpage  staff            96 May  20 13:28 server
```

### About

The Go Programming Language [textbook][gopl] offers a study of concurrency using
[CSP][csp]. One of the exercises, created as a challenge for the reader, is to
implement a concurrent File Transfer Protocol (FTP) server. That is, a server
that allows concurrent connections from multiple clients.

While not a complete implementation of [RFC 959][rfc959], this exercise provides
a survey of commands commonly used when interacting with an FTP server. This
includes `cd` to change a directory, `ls` to list a directory, `get` to
send the contents of a file, `put` to store the contents of a file, and many
[more][more].

As is in the name, simple-ftp-server, I wanted to create a simple FTP server
that could be digested by someone new to the language or looking to build their
own FTP server. Much of the logic implemented is sequential and doesn't shy away
from duplication as [design][design].

### Compile

You will need the latest stable version of go installed on your machine:

https://golang.org/dl/

Run the following commands to download and build for execution:

1. `git clone https://github.com/justinpage/simple-ftp-server.git`
2. `cd simple-ftp-server`
3. `go build -o server main.go`

### Execute

1. Launch ftp server: `./server`
2. Connect: `ftp -P 8080 localhost`
3. Stop ftp server: `Control+C` to interrupt the process with `SIGINT`

### References

- Thanks to D. J. Bernstein for his [material on how FTP actually works][ftp]
- Using the [specification published in 1985][1985] was an experience

[gopl]: http://www.gopl.io/
[csp]: https://en.wikipedia.org/wiki/Communicating_sequential_processes
[rfc959]: https://tools.ietf.org/html/rfc959
[more]: server.go#L94-L137
[ftp]: https://cr.yp.to/ftp.html
[1985]: https://tools.ietf.org/html/rfc959
[design]: https://github.com/justinpage/building-evolutionary-architectures/blob/master/chapter-7.md#antipattern-code-reuse-abuse
