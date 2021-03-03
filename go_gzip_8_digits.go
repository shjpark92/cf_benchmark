package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile `file`")
var nproc = flag.Int("c", 0, "Number of threads")
var duration = flag.Uint("t", 10, "Duration of each benchmark in seconds")
var run = flag.String("r", ".*", "Tests to run")

type benchInit func() func()

var (
	benchEscapeData = strings.Repeat("AAAAA < BBBBB > CCCCC & DDDDD ' EEEEE \" ", 10000)
	textTwain, _ = ioutil.ReadFile("./corp/mt.txt")
	textE, _ = ioutil.ReadFile("./corp/e.txt")
	easyRE          = "ABCDEFGHIJKLMNOPQRSTUVWXYZ$"
	easyREi         =  "(?i)ABCDEFGHIJklmnopqrstuvwxyz$"
        easyRE2         = "A[AB]B[BC]C[CD]D[DE]E[EF]F[FG]G[GH]H[HI]I[IJ]J$"
	mediumRE	=  "[XYZ]ABCDEFGHIJKLMNOPQRSTUVWXYZ$"
	hardRE		=  "[ -~]*ABCDEFGHIJKLMNOPQRSTUVWXYZ$"
	hardRE2		=  "ABCD|CDEF|EFGH|GHIJ|IJKL|KLMN|MNOP|OPQR|QRST|STUV|UVWX|WXYZ"
	text            []byte
)

func makeText(n int) []byte {
	if len(text) >= n {
		return text[:n]
	}
	text = make([]byte, n)
	x := ^uint32(0)
	for i := range text {
		x += x
		x ^= 1
		if int32(x) < 0 {
			x ^= 0x88888eef
		}
		if x%31 == 0 {
			text[i] = '\n'
		} else {
			text[i] = byte(x%(0x7E+1-0x20) + 0x20)
		}
	}
	return text
}

func BenchmarkMatch(re string) func() {
	r := regexp.MustCompile(re)
	size := 1 << 18
	t := makeText(size)

	return func() {
		if r.Match(t) {
			log.Fatalln("Match")
		}
	}
}

var goBenchmarks = []struct {
	name   string
	benc   func() func()
	report func(int) string
}{
	{"compress/gzip compression digits, -8",
		func() func() {
			var b bytes.Buffer
			w, _ := gzip.NewWriterLevel(&b, 8)

			return func() {
				b.Reset()
				w.Reset(&b)
				w.Write(textE)
				w.Flush()
				w.Close()
			}
		},
		func(total int) string {
			return fmt.Sprintf("%.2f MiB/s", float64(total*len(textE))/float64(1024*1024)/float64(*duration))
		},
	},
}



func bench(init benchInit, nThreads int) int {
	var wg sync.WaitGroup
	start := time.Now()
	sum := make(chan int, nThreads)

	wg.Add(nThreads)

	for i := 0; i < nThreads; i++ {
		go func() {
			b := init()
			total := 0

			for time.Now().Sub(start) < time.Duration(*duration)*time.Second {
				b()
				total += 1
			}

			sum <- total
			wg.Done()
		}()
	}

	wg.Wait()
	close(sum)

	total := 0
	for t := range sum {
		total += t
	}

	return total
}

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if *nproc < 0 {
		*nproc = 0
	}

	if *nproc != 0 {
		runtime.GOMAXPROCS(*nproc)
	} else {
		*nproc = runtime.GOMAXPROCS(0)
	}

	log.Println("Max threads:", *nproc, "; CPUs available:", runtime.NumCPU())

	match := regexp.MustCompile(*run)

	for _, b := range goBenchmarks {

		if !match.MatchString(b.name) {
			continue
		}

		totalMulti := bench(b.benc, *nproc)
		fmt.Printf("%s,%s\n", b.name, b.report(totalMulti))
	}
}
