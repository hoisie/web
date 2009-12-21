include $(GOROOT)/src/Make.$(GOARCH)

ALL=hello arcchallenge

all: $(ALL)

clean:
	rm -rf *.6 $(ALL)

%: %.go
	$(GC) $*.go
	$(LD) -o $@ $*.$O
