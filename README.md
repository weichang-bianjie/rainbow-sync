# rainbow-sync
A daemon that synchronizes IRIS hub data for the Rainbow wallet backend

# Structure

- `conf`: config of project
- `block`: parse asset detail and tx function module
- `model`: mongodb script to create database
- `task`: main logic of sync-server, sync data from blockChain and write to database
- `db`: database model
- `msgs`: tx msgs model
- `lib`: cdc and client pool functions
- `utils`: common functions
- `main.go`: bootstrap project

# SetUp
## Database
Use Mongodb  to store IRIS hub data

# Build And Run

- Build: `make all`
- Run: `make run`
- Cross compilation: `make build-linux`

## Run with docker
You can run application with docker.
### Image
- Build Rainbow-sync Image
```$xslt
docker build -t rainbow-sync .
```

- Run Application
```$xslt
docker run --name rainbow-sync \&
-v /mnt/data/rainbow-sync/logs:/root/go/src/github.com/irisnet/rainbow-sync/logs \&
-e "DB_ADDR=127.0.0.1:27217" -e "DB_USER=user" \&
-e "DB_PASSWD=password" -e "DB_DATABASE=db_name"  \&
-e "SER_BC_FULL_NODES=tcp://localhost:26657,..." rainbow-sync
```


## Environment Params

| param | type | default |description | example |
| :--- | :--- | :--- | :---: | :---: |
| DB_ADDR | string | "" | db addr | 127.0.0.1:27017,127.0.0.2:27017... |
| DB_USER | string | "" | db user | user |
| DB_PASSWD | string | "" |db passwd  | password |
| DB_DATABASE | string | "" |database name  | db_name |
| SER_BC_FULL_NODES | string | tcp://localhost:26657 | iris full node rpc url | tcp://localhost:26657, tcp://127.0.0.2:26657 |
| CHAIN_BLOCK_RESET_HEIGHT: `option` `string` block sync reset height; default `0` (example: `1`)

