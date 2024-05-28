EXE_NAME = alpenhorn-config-server

build:
	go build -o $(EXE_NAME) main.go

clean:
	rm $(EXE_NAME)
	rm -rf persist_config_server

init:
	./$(EXE_NAME) -persist ./ -setConfig ../guardian/alpenhorn-guardian-new-config/AddFriend.conf

run:
	sudo ./$(EXE_NAME) -persist ./ -hostname 127.0.0.1:80