# go-ad1
A Go library to read AccessData images (AD1) based on [Petter Christian Bjelland's Python library](https://github.com/pcbje/pyad1)  and [TMairi's blog post](https://tmairi.github.io/posts/dissecting-the-ad1-file-format/) on Dissecting the AD1 File Format.


### TODO

 - Proper error handling
 - Item path building hogs up the memory, because all the items are stored in an array to find parent directories
 - Fragmented image parsing
 - Testing was done using 2-3 images only 

P.S.: I created this library to learn Go. It is my first project written in this language - use with caution.😎
