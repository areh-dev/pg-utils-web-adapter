# pg-utils-web-adapter

Simple web-api adapter to run pg_dump and pg_restore in docker container

Main use case is run this container in multi container apps, that include PostgreSQL database, and call `backup`
and `restore` functions from main application container to include database backup in full app backup

## Storage

All files for backup and restore should be located in `/backups` directory. You should pass stateful volume to this path

## Environment variables

Application tries to load PostgreSQL connection settings from environment variables to run backup and restore command
without additional parameters

- `PG_HOST` - (**required**) PostgreSQL server host
- `PG_PORT` - PostgreSQL server port. Default - 5432
- `PG_DB` - (**required**) database name
- `PG_USER` - (**required**) username 
- `PG_PASS` - user password. Can be empty assuming that the PostgreSQL server is configured to connect without a password

`USE_DIR_STRUCTURE` - ("TRUE") Use directories structure for backups: `/backups/server_name/db_name/backup_file` 

## API usage

- `/status` GET request returns OK if service is up and running
- `/backup` GET for environment config \ POST with JSON config in request
~~- `/backup-db` GET request with full connection settings~~
- `/restore` GET for environment config \ POST with JSON config in request
~~- `/restore-db` GET request with full connection settings~~

## Usage

Include container in your docker-compose.yml

```yaml
services:
  # Container with PostgreSQL server
  pgdb-server:
    image: "postgres:..."
    ...

  pg-util:
    image: "pg-utils-web-adapter:latest"
    container_name: "pg-util-adapter"
    
    volume - /backups
    
    restart: always
    environment:
      - PG_HOST=pgdb-server
      - PG_PORT=5432
      - PG_DB=APP_DATABASE
      - PG_USER=POSTGRES
      - PG_PASS=POSTGRES_PASSWORD 
```

Then call from your main app

- `http://pg-util/backup` to create backup
- `http://pg-util/restore` to restore database from backup