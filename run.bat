Start "PROCESS1" tailwindcss -i input.css -o dist/output.css --watch
go run main -dir test
taskkill /T /FI "WindowTitle eq PROCESS1"