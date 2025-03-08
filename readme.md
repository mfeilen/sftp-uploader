# SFTP-Uploader
SFTP-Uploader is a watchdog that checks for incoming files in a dedicated folder and pushes them to a defined SFTP/SSH-Server

## Features
* supports custom ports
* supports hostkey checks
* supports password auth and priv-key auth
* preferes priv-key auth over password
* Can archive or delete processed files
* Can move failed files in separate folder or keep it in current and ignore them
* Will check if files are still changed before uploading them (intervals adjustable)


## Configuration SFTP-Uploader
Create an `.env` file that contains the following
```
# SFTP server credentails
SFTP_HOST=myserver
SFTP_PORT=22
SFTP_USER=remoteuser
SFTP_PASSWORD='somehardpassword'

# SFTP priv key auth
SFTP_PRIV_KEY_FILE=/home/me/id_rsa.pub
SFTP_PRIV_KEY_PASSWORD=someStrongKeyPassword

# FTPs server credentials
FTPS_HOST=myserver
FTPS_PORT=21
FTPS_USER=remoteuser
FTPS_PASSWORD='somehardpassword'

# FTPs/SFTP Uplaod directory. If no SFTP jail but homedir, use the full path to homeDir, not ~
TARGET_DIR=/uploads

# Use sftp or ftps to upload
CONNECTOR_TYPE=sftp

# directory that shall be checked
WATCH_DIR=/home/me/files/incoming

# Defines the intervall in seconds in which a file size chanage is checked
WATCH_FILE_CHANGE_INTERVAL=2

# Defines when to give up files that may (WATCH_FILE_CHANGE_INTERVAL * WATCH_FILE_MAX_TIME == time wait maximum)
WATCH_FILE_CHANGE_MAX_TIME=5

# move uploaded files to - keep empty if not wanted
ARCHIVE_DIR=/home/me/files/uploaded

# Files are moved that have not been transfered successfully - keep empty if not wanted
FAILED_DIR=/home/me/failed

# delete files after upload instead
DELETE_FILE_AFTER=false

# If not set, remote host key / fingerprint will be ignored
KNOWN_HOSTS=/home/me/.ssh/known_hosts

# Shutdown after x errors on the remote server
SHUT_DOWN_AFTER_ERRORS=5
```

## Configure Logging [romana/log](https://github.com/romana/rlog)
For a better logging experience, create a .env.log
More information about the logging configuration can be found [here](https://github.com/romana/rlog)

A good start could be:
```
# Logging
RLOG_LOG_LEVEL=INFO
RLOG_TIME_FORMAT=RFC3339
RLOG_TRACE_LEVEL=3
```

## Kown issues
If the known_hosts file contains more than one entries for the same IP, the right known host entries cannot be detected. Either use host IP instead or keep KNOWN_HOST empty to disable host key check
