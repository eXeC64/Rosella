Rosella
=======

About
-----
Rosella is a small ircd (Internet Relay Chat Daemon).
It partially implements RFC2812, but will never be fully compliant.

Rosella is intended to provide a portable, light-weight, near-zero-conf
ircd capable of handling many simultaneous connections, whilst providing
as much privacy for its users as possible.

Rosella is *not* production ready, or recommended for large-scale usage.

An x.509 key and certificate are required to open a TLS listener.
Proper key handling and certificate checking is the responsibility of the
users. Rosella cannot protect you from stupidity or untrustworthy CA's.

Usage
-----
Compilation required Go 1.1.2 or later. Just run `make` to compile Rosella.

Command line options can be found by running `ircd -h`.

###x.509 Certificate###
Rosella expects you to provide a valid x.509 certificate and private key.
You can generate these yourself with openssl, or obtain one from a certificate
authority you trust.

###Auth File###
The auth file provides a list of usernames and hashed passwords that the /OPER
command will accept. The format is one username and password pair per line.
Lines starting with a `#` are ignored as comments, as are blank lines. The
password is hashed with SHA1. Username and password are placed on the same
line and separated by a single space, as such:

    #This line is a comment
    username1 sha1_hashed_password

    #Another comment, blank lines are ignored
    username2 sha1_hashed_password
    username3 sha1_hashed_password


Design Principles
-----------------

* Rosella will not spy upon its users, or log them in any way.

* Rosella will not communicate with any users in plaintext.

* Rosella will not provide any mechanism for identifying other users beyond
  their nicknames.

* Rosella will not allow any user to spy upon any other user.

* Rosella's source code will be kept as easy to understand as possible.

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
