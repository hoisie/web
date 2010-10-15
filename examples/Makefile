include $(GOROOT)/src/Make.inc

ALL=hello arcchallenge multipart multiserver params

all: $(ALL)

clean:
	rm -rf *.[68] $(ALL)

%: %.go
	$(GC) $*.go
	$(LD) -o $@ $*.$O
