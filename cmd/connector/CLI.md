
# Klever Connector CLI

The **Klever Term UI** exposes the following Command Line Interface:

```
$ connector --help

NAME:
   Klever Node Connector App - Terminal UI application used to display metrics from the node
USAGE:
   connector [global options]
   
   
GLOBAL OPTIONS:
   --address, -a value       Address and port number on which the application will try to connect to the klever-go node (default: "127.0.0.1:8080")
   --log-level value         This flag specifies the logger level (default: "*:INFO")
   --log-correlation, -c     Will include log correlation elements
   --log-logger-name, -n     Will include logger name
   --interval, -i value      This flag specifies the duration in milliseconds until new data is fetched from the node (default: 1000)
   --use-wss, -w             Will use wss instead of ws when creating the web socket
   --help, -h                show help
   --version, -v             print the version
   

```

