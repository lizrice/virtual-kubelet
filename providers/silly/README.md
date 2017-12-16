# The silly local virtual-kubelet provider

This is kinda useless, but it could become something fun! 

It's a very simple provider for the virtual-kubelet interface that (at least for now) simply keeps a list of its pods. It doesn't actually *do* anything when you create a pod on it other than keep it in a list. You can query the list of current pods on the virtual-kubelet node:

```
$ curl localhost:8080
[{"Name":"silly-pod"}]
```

[![asciicast](https://asciinema.org/a/qt30Dhu0aFvRyA1K9jXIb61H2.png)](https://asciinema.org/a/qt30Dhu0aFvRyA1K9jXIb61H2)

## What's the point?

I'm not sure! 
