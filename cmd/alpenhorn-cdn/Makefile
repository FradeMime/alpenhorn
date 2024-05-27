EXE_NAME = alpenhorn-cdn


build:
	go build -o $(EXE_NAME) main.go

clean:
	rm $(EXE_NAME)
	rm -rf persist_cdn

init:
	./$(EXE_NAME) -persist ../../runConfig/ -init=true

run:
	./$(EXE_NAME) -persist ../../runConfig/