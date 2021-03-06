package parser

import (
	"bufio"
	"github.com/google/syzkaller/pkg/log"
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	maxBufferSize = 64 * 1024 * 1024 // maxBufferSize is maximum size for buffer
	coverDelim    = ","              // Delimiter to split instructions in trace e.g. Cover:0x734,0x735
	coverID       = "Cover:"         // CoverID is the indicator that the line in the trace is the coverage
	sysrestart    = "ERESTART"       // SYSRESTART corresponds to the error code of ERESTART.
	signalPlus    = "+++"            // SignalPlus marks +++
	signalMinus   = "---"            // SignalPlus marks ---
)

func parseCoverage(line string) []uint64 {
	line = line[1 : len(line)-1] //Remove quotes
	ips := strings.Split(strings.Split(line, coverID)[1], coverDelim)
	coverSet := make(map[uint64]bool)
	cover := make([]uint64, 0)
	for _, ins := range ips {
		if strings.TrimSpace(ins) == "" {
			continue
		}
		ip, err := strconv.ParseUint(strings.TrimSpace(ins), 0, 64)
		if err != nil {
			log.Fatalf("failed parsing instruction: %s", ins)
		}
		if _, ok := coverSet[ip]; !ok {
			coverSet[ip] = true
			cover = append(cover, ip)
		}
	}
	return cover
}

func parseSyscall(scanner *bufio.Scanner) (int, *Syscall) {
	lex := newStraceLexer(scanner.Bytes())
	ret := StraceParse(lex)
	return ret, lex.result
}

// ParseLoop parses each line of a strace file in a loop
func ParseLoop(data string) (tree *TraceTree) {
	tree = NewTraceTree()
	//Creating the process tree
	var lastCall *Syscall

	buf := make([]byte, maxBufferSize)
	scanner := bufio.NewScanner(strings.NewReader(data))
	scanner.Buffer(buf, maxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()
		restart := strings.Contains(line, sysrestart)
		signalPlus := strings.Contains(line, signalPlus)
		signalMinus := strings.Contains(line, signalMinus)
		shouldSkip := restart || signalPlus || signalMinus
		if shouldSkip {
			continue
		}
		if strings.Contains(line, coverID) {
			cover := parseCoverage(line)
			log.Logf(4, "Cover: %d", len(cover))
			lastCall.Cover = cover
			continue
		}
		log.Logf(4, "Scanning call: %s", line)
		ret, call := parseSyscall(scanner)
		if ret != 0 {
			log.Fatalf("Error parsing line: %s", line)
		}
		if call == nil {
			log.Fatalf("Failed to parse line: %s", line)
		}
		lastCall = tree.add(call)
	}
	if len(tree.Ptree) == 0 {
		return nil
	}
	return
}

// Parse parses a trace of system calls and returns an intermediate representation
func Parse(filename string) *TraceTree {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("error reading file: %s", err.Error())
	}
	tree := ParseLoop(string(data))
	if tree != nil {
		tree.Filename = filename
	}
	return tree
}
