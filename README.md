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

Design Principles
-----------------

* Rosella will not spy upon its users, or log them in any way.

* Rosella will not communicate with any users in plaintext.

* Rosella will not provide any mechanism for identifying other users beyond
  their nicknames.

* Rosella will not allow any users to spy upon any other users.

* Rosella's source code will remain simple and easy to read to enable quick
  and easy code-reviews by its users.

Contributing
------------

Patches and pull requests are welcome as long as they follow the design
principles I have laid.


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
