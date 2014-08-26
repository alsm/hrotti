package hrotti

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

/*func Test_NewNode(t *testing.T) {
	rootNode := NewNode("test")

	if rootNode == nil {
		t.Fatalf("rootNode is nil")
	}

	if rootNode.Name != "test" {
		t.Fatalf("rootNode name is %s, not test", rootNode.Name)
	}
}*/

func Test_AddSub(t *testing.T) {
	rootNode := NewNode("")
	rand.Seed(time.Now().UnixNano())
	topics := [7]string{"a", "b", "c", "d", "e", "+", "#"}
	for i := 0; i < 20; i++ {
		c := newClient(nil, "testClientId"+strconv.Itoa(i), 100)
		var sub string
		r := rand.Intn(7)
		for j := 0; j <= r; j++ {
			char := topics[rand.Intn(7)]
			sub += char
			if char == "#" || j == r {
				break
			}
			sub += "/"
		}
		rootNode.AddSub(c, strings.Split(sub, "/"), 1)
	}
}

/*func Test_DeleteSub(t *testing.T) {
	rootNode := NewNode("")
	c := newClient(nil, "testClientId", 100)
	sub1 := strings.Split("test/test1/test2/test3", "/")
	sub2 := strings.Split("test/test1/test4/test5", "/")
	complete1 := make(chan byte)
	complete2 := make(chan byte)
	complete3 := make(chan bool)

	rootNode.AddSub(c, sub1, 1, complete1)
	<-complete1
	rootNode.AddSub(c, sub2, 2, complete2)
	<-complete2

	rootNode.Print("")

	rootNode.DeleteSub(c, sub2, complete3)
	<-complete3

	rootNode.Print("")
	close(complete1)
	close(complete2)
	close(complete3)
}

func Test_AddSub2(t *testing.T) {
	c := newClient(nil, "testClientId", 100)
	sub1 := "test/test1/test2/test3"
	sub2 := "test/test1/test4/test5"

	AddSub2(c, sub1, 1)
	AddSub2(c, sub2, 2)
}*/

/*func BenchmarkFindrecip(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	topics := [7]string{"a", "b", "c", "d", "e", "+", "#"}
	for i := 0; i < b.N; i++ {
		c := newClient(nil, "testClientId"+strconv.Itoa(i), 100)
		var sub string
		r := rand.Intn(7)
		for j := 0; j <= r; j++ {
			char := topics[rand.Intn(7)]
			sub += char
			if char == "#" || j == r {
				break
			}
			sub += "/"
		}
		AddSub2(c, sub, 1)
	}
	b.ResetTimer()
	match := FindRecipients2("a/b/c/d/e")
	fmt.Println("a/b/c/d/e", len(match))
}*/

func BenchmarkNormalRouter(b *testing.B) {
	rootNode := NewNode("")
	rand.Seed(time.Now().UnixNano())
	topics := [7]string{"a", "b", "c", "d", "e", "+", "#"}
	for i := 0; i < b.N; i++ {
		c := newClient(nil, "testClientId"+strconv.Itoa(i), 100)
		var sub string
		r := rand.Intn(7)
		for j := 0; j <= r; j++ {
			char := topics[rand.Intn(7)]
			sub += char
			if char == "#" || j == r {
				break
			}
			sub += "/"
		}
		rootNode.AddSub(c, strings.Split(sub, "/"), 1)
	}
	var treeWorkers sync.WaitGroup
	recipients := make(chan *Entry)
	b.ResetTimer()
	treeWorkers.Add(1)
	rootNode.FindRecipients(strings.Split("a/b/c/d/e", "/"), recipients, &treeWorkers)
	treeWorkers.Wait()
	close(recipients)
	for {
		_, ok := <-recipients
		if !ok {
			break
		}
	}
}

func main() {
	br := testing.Benchmark(BenchmarkNormalRouter)
	fmt.Println(br)
}
