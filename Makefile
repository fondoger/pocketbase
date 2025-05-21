
init-postgres:
	docker run -d \
		--name postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=user \
		-e POSTGRES_PASSWORD=pass \
		postgres:alpine
start-postgres:
	docker start postgres


start:
	-docker stop pocketbase
	docker run --rm -d \
		--network=host \
		--name pocketbase \
		-v ./pb_data:/data \
		-v ./pb_public:/pb/pb_public \
		-v ./pb_hooks:/pb/pb_hooks \
		-e PB_HTTP_ADDR=127.0.0.1:8090 \
		-e PB_DATA_DIR="/data" \
		-e PB_HOOKS_DIR="/pb/pb_hooks" \
		-e POSTGRES_URL="postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable" \
		ghcr.io/fondoger/pocketbase:v0.28.2 \
		/pb/pocketbase serve --dev
