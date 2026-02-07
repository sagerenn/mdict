package cache

import (
	"container/list"
	"sync"
	"time"
)

type Cache struct {
	mu   sync.Mutex
	cap  int
	ttl  time.Duration
	ll   *list.List
	data map[string]*list.Element
}

type entry struct {
	key   string
	value any
	exp   time.Time
}

func New(capacity int, ttl time.Duration) *Cache {
	if capacity <= 0 {
		capacity = 256
	}
	return &Cache{
		cap:  capacity,
		ttl:  ttl,
		ll:   list.New(),
		data: make(map[string]*list.Element),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.data[key]; ok {
		e := ele.Value.(*entry)
		if c.ttl > 0 && time.Now().After(e.exp) {
			c.ll.Remove(ele)
			delete(c.data, key)
			return nil, false
		}
		c.ll.MoveToFront(ele)
		return e.value, true
	}
	return nil, false
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.data[key]; ok {
		c.ll.MoveToFront(ele)
		e := ele.Value.(*entry)
		e.value = value
		e.exp = expiry(c.ttl)
		return
	}
	el := c.ll.PushFront(&entry{key: key, value: value, exp: expiry(c.ttl)})
	c.data[key] = el
	if c.ll.Len() > c.cap {
		last := c.ll.Back()
		if last != nil {
			c.ll.Remove(last)
			le := last.Value.(*entry)
			delete(c.data, le.key)
		}
	}
}

func expiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}
