package pbservice

import "viewservice"
import "net/rpc"
import "fmt"
import "sync"

// You'll probably need to uncomment these:
// import "time"
// import "crypto/rand"
// import "math/big"

import "crypto/rand"
import "math/big"
func nrand() int64 {
  max := big.NewInt(int64(1) << 62)
  bigx, _ := rand.Int(rand.Reader, max)
  x := bigx.Int64()
  return x
}

type Clerk struct {
	vs *viewservice.Clerk
	// Your declarations here
	view viewservice.View
	lock sync.Mutex
	fifoLock sync.Mutex
	lastReqProcessed int
	nextReqQueued int
}

func MakeClerk(vshost string, me string) *Clerk {
	ck := new(Clerk)
	ck.vs = viewservice.MakeClerk(me, vshost)
	// Your ck.* initializations here
	return ck
}

//
// call() sends an RPC to the rpcname handler on server srv
// with arguments args, waits for the reply, and leaves the
// reply in reply. the reply argument should be a pointer
// to a reply structure.
//
// the return value is true if the server responded, and false
// if call() was not able to contact the server. in particular,
// the reply's contents are only valid if call() returned true.
//
// you should assume that call() will time out and return an
// error after a while if it doesn't get a reply from the server.
//
// please use call() to send all RPCs, in client.go and server.go.
// please don't change this function.
//
func call(srv string, rpcname string,
	args interface{}, reply interface{}) bool {
	c, errx := rpc.Dial("unix", srv)
	if errx != nil {
		return false
	}
	defer c.Close()

	err := c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

//	fmt.Println(err)
	return false
}


//
// fetch a key's value from the current primary;
// if they key has never been set, return "".
// Get() must keep trying until it either the
// primary replies with the value or the primary
// says the key doesn't exist (has never been Put().
//
func (ck *Clerk) Get(key string) string {
	ck.fifoLock.Lock()
	myCount := ck.nextReqQueued
	ck.nextReqQueued += 1
	ck.fifoLock.Unlock()

	for ; ; {
		ck.fifoLock.Lock()
		if (ck.lastReqProcessed >= myCount)  {
			ck.fifoLock.Unlock()
			break
		}
		ck.fifoLock.Unlock()		
	}
	ck.lock.Lock()
	args := GetArgs{Key:key, Token:nrand(), Viewnum:ck.vs.Viewnum()}
	var reply GetReply	
	callGetExactlyOnce(ck.vs.Primary, args, &reply, ck)
	if (reply.Err != "ok")  {
		fmt.Println(reply.Err, args, ck.vs.Primary(), ck.vs.Viewnum())
	}
	ck.lastReqProcessed += 1 
	ck.lock.Unlock()
	return reply.Value
}

//
// tell the primary to update key's value.
// must keep trying until it succeeds.
//
func (ck *Clerk) PutExt(key string, value string, dohash bool) string {	
	ck.fifoLock.Lock()
	myCount := ck.nextReqQueued
	ck.nextReqQueued += 1
	ck.fifoLock.Unlock()

	for ; ; {
		ck.fifoLock.Lock()
		if (ck.lastReqProcessed >= myCount)  {
			ck.fifoLock.Unlock()
			break
		}
		ck.fifoLock.Unlock()		
	}
	//value = strconv.FormatUint(uint64(hash(ck.Get(key) + value)), 10)
	ck.lock.Lock()	
	args := PutArgs{Key:key, Value:value, DoHash:dohash, Token:nrand(), Viewnum:ck.vs.Viewnum()}
	var reply PutReply
	callPutExactlyOnce(ck.vs.Primary, args, &reply)
	ck.lastReqProcessed += 1 
	ck.lock.Unlock()
	return reply.PreviousValue
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutExt(key, value, false)
}
func (ck *Clerk) PutHash(key string, value string) string {
	v := ck.PutExt(key, value, true)
	return v
}
