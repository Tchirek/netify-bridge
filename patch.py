import re
src = open('main.go').read()
old = """        if len(addr) > 6 && addr[:6] == \"unix://\" {
            c, err = net.Dial(\"unix\", addr[7:])
        } else {
            c, err = net.DialTimeout(\"tcp\", addr, 5*time.Second)
        }"""
new = """        target := addr
        if i := strings.Index(target, \"://\"); i > 0 {
            target = target[i+3:]
        }
        c, err = net.DialTimeout(\"tcp\", target, 5*time.Second)"""
assert old in src, "pattern not found"
src = src.replace(old, new)
src = src.replace('import (', 'import (\n    \"strings\"')
open('main.go','w').write(src)
print("patched ok")
