from fabric import Connection, task
from invoke import run

@task
def deploy(c):
    with Connection('alicloud','root') as c:
        c.run("docker pull snoooowwhite/checkin:1.0.0", pty=True)
        c.put("docker-compose.yml", "/root/checkin/docker-compose.yml")
        c.run("cd checkin && docker-compose -f docker-compose.yml up -d", pty=True)
        c.run("docker logs -f checkin")

@task
def init(c):
    run("env GOOS=linux GOARCH=amd64 go build")
    run("docker build -t snoooowwhite/checkin:1.0.0 .")
    run("docker push snoooowwhite/checkin:1.0.0")
