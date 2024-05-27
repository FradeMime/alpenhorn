EXE_NAME = alpenhorn-guardian-keygen

build:
	go build -o $(EXE_NAME) main.go

clean:
	rm $(EXE_NAME)
	rm ~/.alpenhorn/guardian.privatekey
	rm ~/.alpenhorn/guardian.publickey

run:
	./$(EXE_NAME)

log:
	@echo "密钥默认保存路径：~/.alpenhorn/"