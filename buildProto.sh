if [ -v $GOPATH ]; then
   GOPATH=~/go
fi

PROJECTROOT=$(pwd)

find * -name "proto" -print0 | while read -d $'\0' protofolder
do
    fullPath="$PROJECTROOT/$protofolder"
    basePath="$PROJECTROOT/$(dirname ${protofolder})"
    echo Building protos on "$fullPath"
    find $fullPath -name "*.proto" -type f -print0 | while read -d $'\0' file
    do
        echo $(basename ${file})
        protoc -I=$fullPath -I=$GOPATH -I=. -I=$PROJECTROOT/protobuf --go_out=$basePath $file
    done
    
done
