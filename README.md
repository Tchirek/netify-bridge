# netify-bridge

1. 校园网被白嫖，想要民办西比拉
2. 向 agent 许愿，选型颇为神奇
3. 编译 v5 版本 aarch64
4. 写了个面板
5. 发现不如 OAF，造了个轮子
6. 记之而去

---

netifyd JSON sink 桥 + 极简网页面板。217 行 Go，跑在 OpenWrt aarch64 路由器上。

```sh
go build
./netify-bridge -sink tcp://<router>:1750 -listen :8080
```

面板：`http://<router>:8080`