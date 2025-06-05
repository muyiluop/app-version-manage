#!/bin/sh
/app/app &
nginx -g 'daemon off;'
