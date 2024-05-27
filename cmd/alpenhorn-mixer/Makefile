EXE_NAME = alpenhorn-mixer

build:
	go build -o $(EXE_NAME) main.go

clean:
	rm $(EXE_NAME)

init:
	./$(EXE_NAME) -persist ../../runConfig/ -init=true

run:
	./$(EXE_NAME) -persist ../../runConfig/