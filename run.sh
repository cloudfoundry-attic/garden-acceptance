#!/bin/bash - 
#===============================================================================
#
#          FILE: run.sh
# 
#         USAGE: ./run.sh 
# 
#   DESCRIPTION: 
# 
#       OPTIONS: ---
#  REQUIREMENTS: ---
#          BUGS: ---
#         NOTES: ---
#        AUTHOR: YOUR NAME (), 
#  ORGANIZATION: 
#       CREATED: 02/11/2014 12:23
#      REVISION:  ---
#===============================================================================

set -o nounset                              # Treat unset variables as an error
set -e

clear

echo "Running..." 

ginkgo -succinct=true -slowSpecThreshold=150

