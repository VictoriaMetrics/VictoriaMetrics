prepare-logs:
	cd ./source_logs && bash download.sh

docker-up-elk:
	docker-compose -f docker-compose.yml -f docker-compose-elk.yml up -d

docker-stop-elk:
	docker-compose -f docker-compose.yml -f docker-compose-elk.yml stop

docker-up-loki:
	docker-compose -f docker-compose.yml -f docker-compose-loki.yml up -d

docker-stop-loki:
	docker-compose -f docker-compose.yml -f docker-compose-loki.yml stop

docker-cleanup:
	docker-compose -f docker-compose.yml -f docker-compose-elk.yml -f docker-compose-loki.yml down -v --remove-orphans
