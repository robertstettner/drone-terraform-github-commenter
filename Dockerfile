FROM cloudposse/github-commenter

RUN apk add --no-cache bash
ADD ./commenter /usr/bin
ENTRYPOINT [ "commenter" ]