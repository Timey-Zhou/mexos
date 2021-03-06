package shardkv

import "shardmaster"
import "net/rpc"
import "time"
import "sync"
import "fmt"

type Clerk struct {
	mu       sync.Mutex // one RPC at a time
	sm       *shardmaster.Clerk
	config   shardmaster.Config
	me       int64
	network  bool
	clientID int64
}

func MakeClerk(shardmasters []string, network bool) *Clerk {
	ck := new(Clerk)
	ck.sm = shardmaster.MakeClerk(shardmasters, network)
	ck.me = nrand()
	ck.network = network
	ck.clientID = nrand()
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
func call(srv string, rpcname string, args interface{},
	reply interface{}, network bool) bool {
	if network {
		c, errx := rpc.Dial("tcp", srv)
		if errx != nil {
			return false
		}
		defer c.Close()

		err := c.Call(rpcname, args, reply)
		if err == nil {
			return true
		}

		if printRPCerrors {
			fmt.Println(err)
		}
		return false

	} else {
		c, errx := rpc.Dial("unix", srv)
		if errx != nil {
			return false
		}
		defer c.Close()

		err := c.Call(rpcname, args, reply)
		if err == nil {
			return true
		}

		if printRPCerrors {
			fmt.Println(err)
		}
		return false
	}
}

//
// which shard is a key in?
// please use this function,
// and please do not change it.
//
func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= shardmaster.NShards
	return shard
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
func (ck *Clerk) Get(key string) string {
	ck.mu.Lock()
	defer ck.mu.Unlock()

	shard := key2shard(key)
	args := &GetArgs{key, nrand(), ck.clientID}

	for {
		gid := ck.config.Shards[shard]

		servers, ok := ck.config.Groups[gid]

		if ok {
			// try each server in the shard's replication group.
			for _, srv := range servers {
				var reply KVReply
				ok := call(srv, "ShardKV.Get", args, &reply, ck.network)
				if ok && (reply.Err == OK || reply.Err == ErrNoKey) {
					return reply.Value
				}
				if ok && (reply.Err == ErrWrongGroup) {
					break
				}
			}
		}

		time.Sleep(50 * time.Millisecond)

		// ask master for a new configuration.
		ck.config = ck.sm.Query(-1)
	}
	return ""
}

func (ck *Clerk) PutExt(key string, value string, dohash bool) string {
	ck.mu.Lock()
	defer ck.mu.Unlock()
	DPrintf("got put")
	shard := key2shard(key)
	args := &PutArgs{key, value, dohash, nrand(), ck.clientID}

	for {
		gid := ck.config.Shards[shard]

		servers, ok := ck.config.Groups[gid]
		DPrintf("Shards replication group is %v", servers)
		DPrintf("Overall shard assignment is %v", ck.config.Groups)
		if ok {
			// try each server in the shard's replication group.
			for _, srv := range servers {
				var reply KVReply
				DPrintf("About to send Put rpc to %s.", srv)
				ok := call(srv, "ShardKV.Put", args, &reply, ck.network)
				if ok && reply.Err == OK {
					return reply.Value
				}
				if ok && (reply.Err == ErrWrongGroup) {
					DPrintf("Err wrong group")
					break
				}
			}

		}
		time.Sleep(50 * time.Millisecond)

		// ask master for a new configuration.
		prior := ck.config.Num
		ck.config = ck.sm.Query(-1)
		DPrintf("Prior config %d new is %d: %v", prior, ck.config.Num,
			ck.config.Shards,)
	}
	return ""
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutExt(key, value, false)
}
func (ck *Clerk) PutHash(key string, value string) string {
	v := ck.PutExt(key, value, true)
	return v
}
