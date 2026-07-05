package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Instance 下游服务实例
type Instance struct {
	Addr string // host:port
}

// Discovery 基于 etcd 的服务发现器
type Discovery struct {
	client       *clientv3.Client
	serviceNames []string
	mu           sync.RWMutex
	// serviceName → 实例列表
	services map[string][]Instance
	// serviceName → 轮询计数
	rrIdx map[string]int
}

// New 创建服务发现器并连接 etcd（连接为懒加载，etcd 暂时不可用不会失败）
// serviceNames 为需要发现的服务名列表（对应各服务 yaml 中 Etcd.Key 的值）
func New(etcdHosts []string, serviceNames []string) (*Discovery, error) {
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("etcd hosts 为空")
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdHosts,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("连接 etcd 失败: %w", err)
	}
	return &Discovery{
		client:       client,
		serviceNames: serviceNames,
		services:     make(map[string][]Instance),
		rrIdx:        make(map[string]int),
	}, nil
}

// Watch 启动后台 Watch goroutine。
// 先从 etcd 拉取一次现有服务列表（失败仅记录日志），再为每个服务开启 Watch 监听变更。
// 返回初始加载错误，调用方可据此日志告警；Watch goroutine 始终启动并自动重连。
func (d *Discovery) Watch(ctx context.Context) error {
	loadErr := d.loadAll(ctx)
	if loadErr != nil {
		logx.Errorf("discovery 初始加载服务列表失败，将通过 Watch 重试: %v", loadErr)
	}
	// 为每个服务启动独立的 Watch goroutine
	for _, name := range d.serviceNames {
		go d.watchService(ctx, name)
	}
	return loadErr
}

// Close 关闭 etcd 连接
func (d *Discovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}

// loadAll 一次性拉取所有服务的实例列表
// go-zero 注册的 etcd key 格式为 <service-name>/<addr>，例如 auth.service/127.0.0.1:8081
func (d *Discovery) loadAll(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	total := 0
	for _, name := range d.serviceNames {
		prefix := name + "/"
		resp, err := d.client.Get(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			logx.Errorf("拉取服务 %s 实例列表失败: %v", name, err)
			continue
		}
		for _, kv := range resp.Kvs {
			serviceName, addr := parseServiceKey(string(kv.Key), string(kv.Value))
			if serviceName == "" || addr == "" {
				continue
			}
			// 去重
			exists := false
			for _, ins := range d.services[serviceName] {
				if ins.Addr == addr {
					exists = true
					break
				}
			}
			if !exists {
				d.services[serviceName] = append(d.services[serviceName], Instance{Addr: addr})
				total++
			}
		}
	}
	logx.Infof("discovery 初始加载完成，服务数=%d 实例数=%d", len(d.services), total)
	return nil
}

// watchService 监听单个服务的实例变更，断开后自动重连
func (d *Discovery) watchService(ctx context.Context, serviceName string) {
	prefix := serviceName + "/"
	for {
		if ctx.Err() != nil {
			return
		}
		rch := d.client.Watch(ctx, prefix, clientv3.WithPrefix())
		for resp := range rch {
			if resp.Err() != nil {
				logx.Errorf("discovery watch %s 错误: %v", serviceName, resp.Err())
				break
			}
			d.mu.Lock()
			for _, ev := range resp.Events {
				switch ev.Type {
				case clientv3.EventTypePut:
					d.addInstanceLocked(string(ev.Kv.Key), string(ev.Kv.Value))
				case clientv3.EventTypeDelete:
					d.removeInstanceLocked(string(ev.Kv.Key))
				}
			}
			d.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// addInstanceLocked 新增或更新一个实例（调用方持锁）
func (d *Discovery) addInstanceLocked(key, value string) {
	serviceName, addr := parseServiceKey(key, value)
	if serviceName == "" || addr == "" {
		return
	}
	for _, ins := range d.services[serviceName] {
		if ins.Addr == addr {
			return
		}
	}
	d.services[serviceName] = append(d.services[serviceName], Instance{Addr: addr})
	logx.Infof("discovery 添加实例 service=%s addr=%s 当前实例数=%d",
		serviceName, addr, len(d.services[serviceName]))
}

// removeInstanceLocked 移除一个实例（调用方持锁）
func (d *Discovery) removeInstanceLocked(key string) {
	serviceName, addr := parseServiceKey(key, "")
	if serviceName == "" {
		return
	}
	instances := d.services[serviceName]
	for i, ins := range instances {
		if ins.Addr == addr || addr == "" {
			d.services[serviceName] = append(instances[:i], instances[i+1:]...)
			remaining := len(d.services[serviceName])
			if remaining == 0 {
				delete(d.services, serviceName)
				delete(d.rrIdx, serviceName)
			}
			logx.Infof("discovery 移除实例 service=%s addr=%s 剩余实例数=%d",
				serviceName, ins.Addr, remaining)
			return
		}
	}
}

// GetInstances 返回某服务的全部实例快照
func (d *Discovery) GetInstances(serviceName string) []Instance {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]Instance(nil), d.services[serviceName]...)
}

// Pick 轮询选择一个实例，无可用实例时 ok=false
func (d *Discovery) Pick(serviceName string) (Instance, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	instances := d.services[serviceName]
	if len(instances) == 0 {
		return Instance{}, false
	}
	idx := d.rrIdx[serviceName]
	d.rrIdx[serviceName] = idx + 1
	return instances[idx%len(instances)], true
}

// parseServiceKey 从 etcd key 解析 serviceName 与 addr
// go-zero 注册的 key 格式：<service-name>/<addr>，例如 auth.service/127.0.0.1:8081
// value 通常为 addr 字符串，value 非空时优先取 value 作为 addr，否则回退到 key 末段
func parseServiceKey(key, value string) (serviceName, addr string) {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return key, ""
	}
	serviceName = key[:idx]
	if value != "" {
		addr = value
	} else {
		addr = key[idx+1:]
	}
	return serviceName, addr
}
