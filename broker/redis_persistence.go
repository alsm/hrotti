package hrotti

import (
	"github.com/garyburd/redigo/redis"
	"time"
)

type RedisPersistence struct {
	pool *redis.Pool
}

func NewRedisPersistence(server, password string) *RedisPersistence {
	r := &RedisPersistence{}
	r.pool = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	return r
}

func (r *RedisPersistence) Open(c *Client) {
}

func (r *RedisPersistence) Close(c *Client) {
	handle := r.pool.Get()
	defer handle.Close()
	handle.Send("DEL", "setname")
	handle.Flush()
}

func (r *RedisPersistence) Add(c *Client, cp ControlPacket) bool {
	handle := r.pool.Get()
	defer handle.Close()
	_, err := handle.Do("HSET", c.clientID, cp.MsgID(), cp.Pack())
	if err != nil {
		return false
	}
	return true
}

func (r *RedisPersistence) Replace(c *Client, cp ControlPacket) bool {
	handle := r.pool.Get()
	defer handle.Close()
	_, err := handle.Do("HSET", c.clientID, cp.MsgID(), cp.Pack())
	if err != nil {
		return false
	}
	return true
}

func (r *RedisPersistence) AddBatch(messages map[*Client]*publishPacket) {
	handle := r.pool.Get()
	defer handle.Close()
	handle.Send("MULTI")
	for c, cp := range messages {
		handle.Send("HSET", c.clientID, cp.MsgID(), cp.Pack())
	}
	_, err := handle.Do("EXEC")
	if err != nil {
		return
	}
	return
}

func (r *RedisPersistence) Delete(c *Client, id msgID) bool {
	handle := r.pool.Get()
	defer handle.Close()
	_, err := handle.Do("HDEL", c.clientID, id)
	if err != nil {
		return false
	}
	return true
}

func (r *RedisPersistence) GetAll(c *Client) []ControlPacket {
	var returnPackets []ControlPacket
	handle := r.pool.Get()
	defer handle.Close()
	values, err := redis.Values(handle.Do("HGETALL", c.clientID))
	if err != nil {
		return nil
	}
	for _, value := range values {
		cp := New(value.([]byte)[0])
		cp.Unpack(value.([]byte))
		returnPackets = append(returnPackets, cp)
	}
	return returnPackets
}

func (r *RedisPersistence) Exists(c *Client) bool {
	handle := r.pool.Get()
	defer handle.Close()
	value, err := redis.Uint64(handle.Do("HLEN", c.clientID))
	if err != nil || value == 0 {
		return false
	}
	return true
}
