package main

/*

Plan for testing:
	1. Restore DB from mock-dump
	2. Make some changes in DB + make backup

backup & restore with errors:
	- not all utils in package (?)
	- not configured access without password
	- not correct password for DB
	- not correct connection settings (unknown server \ port \ ...)

*/
