checkin 
==========

checkin is a program for check in and claim rewards in Fox.ONE

## usage

- Copy `config.example.yml` to `config.yml`, add your owner information, multiple accounts are allowed.

- Change the `Dockerfile`, `docker-compose.yml`, `fabfile.py` as you like. You must replace the variable `snoooowwhite/checkin:1.0.0` with your owner docker image name, then fill in your server's name and authorized user. 

- Run `fab deploy`. Have fun!