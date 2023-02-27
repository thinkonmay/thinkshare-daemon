@echo off

set SERVICE_NAME=thinkremotesvc

net stop %SERVICE_NAME%

sc delete %SERVICE_NAME%
