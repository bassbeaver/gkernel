# gkernel

Simple Golang event-driven framework for web applications.

### Installation

With go mod
 ```bash
 go get github.com/bassbeaver/gkernel
 ```

With Dep:
 ```bash
 dep ensure --add github.com/bassbeaver/gkernel
 ```
 
 ### Concepts
 
 Main idea of this framework is to organize request processing flow which is: 
 * controlled by events
 * processed by services, and services are managed by Service Container

Main structure blocks of **gkernel** are:
* **Service Container** or just **Container**, entity to provide service location and [dependency injection](https://wikipedia.org/wiki/Dependency_injection). [bassbeaver/gioc](https://github.com/bassbeaver/gioc) is used for it.
* **Event** - instance indicating that something has happened in system. **Events** are processed via **event buses** and **event listeners**.
* **Controller** - instance to process request.
* **Route** - instance describing bindings between request (http method, url) and controller. Also route has its own event bus. 
* **Response** - instance, encapsulating data, that should be sent to user as a result of request

Also **gkernel** provides simple and convenient way to configure all of this things using config file in yaml  format.
