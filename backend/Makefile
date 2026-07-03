run:
	@pkill -9 -f "supportiq" 2>/dev/null; true
	@go build -o /tmp/supportiq-api ./cmd/ && go build -o /tmp/supportiq-worker ./cmd/worker/
	@/tmp/supportiq-api > /tmp/api.log 2>&1 &
	@/tmp/supportiq-worker > /tmp/worker.log 2>&1 &
	@sleep 3 && curl -s http://localhost:8080/api/v1/health && echo ""

stop:
	@pkill -9 -f "supportiq" 2>/dev/null; echo "Stopped"

.PHONY: run stop
