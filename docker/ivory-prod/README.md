## Docker environment for production

You can **build** and **run** project locally by executing this commands.

1. `make build` - build binary for service (in _service_ package)
2. `yarn run build` or `npm run build` - build web project (in _web_ package)
3. `docker build -t ivory .` - create image for docker (in _root_ package)
4. `mkdir -p docker/ivory-prod/data` - create host folder for persistent data
5. Linux/macOS: `docker run -d -p 80:80 --name ivory --mount type=bind,source="$(pwd)/docker/ivory-prod/data",target=/opt/ivory/data ivory` - run docker container with external data folder
6. PowerShell: `docker run -d -p 80:80 --name ivory --mount "type=bind,source=${PWD}/docker/ivory-prod/data,target=/opt/ivory/data" ivory`

Or run with docker compose (recommended):

```bash
cd docker/ivory-prod
docker compose up -d --build
```

Please, be aware that it won't work with **development environment**
while you do not connect this container to network of the environment. You can do that by executing this command

7. (optional) `docker network connect development_dev-patroni ivory` - connect to dev environment (`patroni[1-3]:[8001-8003]`)

Dockerfile is located in root path, because of docker restrictions

In production, it will work only with full domains name like _google.com_, just _google_ won't work cause container
doesn't know anything about your local machine network

### Environment variables

- `IVORY_URL_PATH` - can be set by user, `default: /`
- `IVORY_STATIC_FILES_PATH` - set in entrypoint.sh file
- `IVORY_CLUSTERS_FILE_PATH` - path to clusters config (folder or single JSON file), `default: data/config/clusters`
- `IVORY_VERSION_TAG` - should be set by build system
- `IVORY_VERSION_COMMIT` - should be set by build system

### Clusters Config Folder

By default Ivory reads clusters from `data/config/clusters` on every request (no restart required).
Each file with mask `*-cluster.json` is loaded and merged into one config in memory.

Example file `data/config/clusters/example-cluster.json`:

```json
{
  "name": "example",
  "sidecars": [
    {"host": "172.22.0.15", "port": 8013},
    {"host": "172.22.0.16", "port": 8013}
  ],
  "tls": {"sidecar": false, "database": false},
  "certs": {},
  "credentials": {
    "patroni": {"username": "patroni", "password": "your-patroni-password"},
    "postgres": {"username": "postgres", "password": "your-postgres-password"}
  },
  "tags": ["all"]
}
```

Legacy single-file format (`clusters.json`) is still supported for backward compatibility.
