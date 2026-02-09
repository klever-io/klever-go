
# Keygenerator CLI

The **Key generation Tool** exposes the following Command Line Interface:

```
$ keygenerator --help

NAME:
   Key generation Tool - This binary will generate a validatorKey.pem and walletKey.pem, each containing private key(s)
USAGE:
   keygenerator [global options]
   
AUTHOR:
   KleverIO <contact@klever.org>
   
GLOBAL OPTIONS:
   --num-keys, -n value       How many keys should generate. Example: 1 (default: 1)
   --key-type, -t value       What kind of keys should generate. Available options: validator, wallet, both (default: "validator")
   --console-out, -c          Boolean option that will enable printing the generated keys directly on the console
   --no-split, -s             Boolean option that will make each generated key added in the same file
   --password, -p value       Password encryption for generated file. Example: --password=MY_SECRET
   --password-file value      Path to a file containing the password for encryption
   --help, -h                 show help
   --version, -v              print the version
   

```

