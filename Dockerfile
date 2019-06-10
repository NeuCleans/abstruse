# Stage 1 image

# FROM arm64v8/node:alpine as base # aarch64
FROM bern/alpine-node-dns:10 as base

ENV DOCKER_VERSION=18.03.1-ce

#ENV ARCH=aarch64 # aarch64
ENV ARCH=x86_64

RUN apk --no-cache add openssl \
  && wget https://download.docker.com/linux/static/stable/$ARCH/docker-$DOCKER_VERSION.tgz -O /tmp/docker.tgz \
  && mkdir /tmp/docker && tar xzf /tmp/docker.tgz -C /tmp \
  && ln -s /tmp/docker/docker /usr/bin/docker && chmod 755 /usr/bin/docker && rm -rf /tmp/docker.tgz \
  && apk del openssl && npm install webpack -g


# Stage 2 image
FROM base as build

WORKDIR /app

COPY package.json package-lock.json tsconfig.json webpack.*.js angular.json /app/
COPY src /app/src

RUN apk add --no-cache --virtual .build-dependencies make gcc g++ python curl sqlite git

RUN npm set progress=false && npm config set depth 0
RUN npm install --only=production
RUN cp -R node_modules prod_node_modules
RUN npm install
RUN npm run build:prod
RUN ls -lha /usr/lib/node_modules
RUN apk del .build-dependencies


# Stage 3 image
FROM bern/alpine-dns:3.7

ARG VCS_REF=n/a
ARG VERSION=dev
ARG BUILD_DATE=n/a

LABEL maintainer="Bernard Ojengwa <bernardojengwa@gmail.com>" \
  org.label-schema.schema-version="1.0" \
  org.label-schema.name="abstruse" \
  org.label-schema.description="Continuous integration platform, simple, scalable and fast deployed on Kubernetes" \
  org.label-schema.url="https://abstruse.bleenco.io/" \
  org.label-schema.vcs-url="https://github.com/neucleans/abstruse" \
  org.label-schema.vendor="Bleenco" \
  org.label-schema.vcs-ref=$VCS_REF \
  org.label-schema.version=$VERSION \
  org.label-schema.build-date=$BUILD_DATE

WORKDIR /app

RUN apk --no-cache add tini sqlite git wget

COPY --from=base /usr/bin/node /usr/bin
COPY --from=base /usr/lib/libgcc* /usr/lib/libstdc* /usr/lib/
COPY --from=base /tmp/docker/docker /usr/bin/docker

COPY --from=build /app/package.json /app/
COPY --from=build /app/prod_node_modules /app/node_modules
COPY --from=build /app/dist /app/dist
COPY --from=build /app/src/files /app/src/files

RUN npm install --save-dev webpack
HEALTHCHECK --interval=10s --timeout=2s --start-period=20s \
  CMD wget -q -O- http://localhost:6500/status || exit 1

EXPOSE 6500

ENTRYPOINT ["/sbin/tini", "--"]
CMD [ "node", "dist/api/index.js" ]
