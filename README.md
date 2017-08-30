Rosella
=======

About
-----
Rosella is a small ircd (Internet Relay Chat Daemon).
It partially implements RFC1459, but will never be fully compliant.

Rosella is intended to provide a portable, light-weight, near-zero-conf
ircd capable of handling many simultaneous connections, whilst providing
as much privacy for its users as possible.

Rosella *only* communicates with clients over SSL/TLS connections, therefore an
x.509 certificate and private key are required for operation. Proper key
handling and certificate checking is the responsibility of the users. Rosella
cannot protect you from stupidity or untrustworthy CA's.

Features
--------

Rosella is a stand-alone server, and does not support server→server
communication or services→server communication. Basic services are expected to
be provided by IRC bots.

The following channel modes are supported:

* s - Secret. The channel is hidden from /LIST unless you are already in it.
* n - No external. Only users in the channel may send messages to it.
* t - Topic Locked. Only operators may set the topic.
* m - Moderated. Only users with voice or operators may talk.

The following irc commands are supported:

* INFO
* JOIN
* KICK
* KILL
* LIST
* MODE
* NICK
* OPER
* PART
* PRIVMSG
* QUIT
* TOPIC
* USER
* VERSION

Building
--------

To fetch the source code, ensure you have Go 1.1.2 or later installed, and your
`$GOPATH` properly configured.
~~~
go get github.com/eXeC64/Rosella
cd $GOPATH/src/github.com/eXeC64/Rosella
~~~

You can then browse and review the source code at your leisure before compiling
it by running `go build`.

Usage
-----
Command line options can be found by running `Rosella -h`.

### x.509 Certificate ###
Rosella expects you to provide a valid x.509 certificate and private key.
You can generate these yourself with openssl, or obtain one from a certificate
authority you trust.

### Auth File ###
The auth file provides a list of usernames and hashed passwords that the /OPER
command will accept. The format is one username and password pair per line.
Lines starting with a `#` are ignored as comments, as are blank lines. The
password is hashed with bcrypt. Username and password are placed on the same
line and separated by a single space, as such:

    #This line is a comment
    username1 bcrypt_hashed_password

    #Another comment, blank lines are ignored
    username2 bcrypt_hashed_password
    username3 bcrypt_hashed_password

**Treat this file as you would treat a private key file.**

Design Principles
-----------------

* Rosella will not spy upon its users, or log them in any way.

* Rosella will not communicate with any users in plaintext.

* Rosella will not provide any mechanism for identifying other users beyond
  their nicknames.

* Rosella will not allow any user to spy upon any other user.

* Rosella's source code will be kept as easy to review as possible.

Contributing
------------

Patches and pull requests for new features and code clean ups are welcome as
long as they follow the design principles.

License
-------

    Copyright (C) 2013 Harry Jeffery

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as
    published by the Free Software Foundation, either version 3 of the
    License, or (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
