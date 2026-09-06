# 本文件定义 Blockscout 区块链浏览器工具镜像,从固定官方源码重建 Web 与索引器。
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required
ARG BLOCKSCOUT_BUILD_IMAGE=hexpm/elixir@sha256:31a23456941cff0f9b2aa5083e42d3371538e6e7032fc49185f6f4e87bcaacdd
ARG BLOCKSCOUT_SOURCE_URL=https://github.com/blockscout/blockscout/archive/refs/tags/v11.2.8.tar.gz
ARG BLOCKSCOUT_SOURCE_SHA256=223e504a3fd48157b90e0a6fb4ac6dc6432b7acfb19cbf7e0e99d7798413cf5e

FROM ${BLOCKSCOUT_BUILD_IMAGE} AS blockscout-deps

ARG BLOCKSCOUT_SOURCE_URL
ARG BLOCKSCOUT_SOURCE_SHA256
ENV MIX_ENV=prod
ENV MIX_HOME=/opt/mix
ENV DISABLE_WEBAPP=false
ENV ADMIN_PANEL_ENABLED=false
# Blockscout does not use OpenCV contrib modules; use the exact NIF artifact required by v11.2.8.
ENV EVISION_ENABLE_CONTRIB=false
ENV ELIXIR_MAKE_CACHE_DIR=/tmp/evision-cache

USER root
WORKDIR /app
ADD ${BLOCKSCOUT_SOURCE_URL} /tmp/blockscout-source.tar.gz
RUN echo "${BLOCKSCOUT_SOURCE_SHA256}  /tmp/blockscout-source.tar.gz" | sha256sum -c - \
    && apk --no-cache --update add alpine-sdk gmp-dev automake libtool inotify-tools autoconf python3 file gcompat libstdc++ curl ca-certificates git make bash \
    && tar -xzf /tmp/blockscout-source.tar.gz -C /app --strip-components=1 \
    && rm -f /tmp/blockscout-source.tar.gz \
    && sed -i \
        -e '/{BlocksIndexedCounter, name: BlocksIndexedCounter},/d' \
        -e '/{InternalTransactionsIndexedCounter, name: InternalTransactionsIndexedCounter},/d' \
        -e 's/{Absinthe.Subscription, Endpoint}/{Absinthe.Subscription, Endpoint},/' \
        -e '/{Absinthe.Subscription, Endpoint}/a\\          {BlocksIndexedCounter, name: BlocksIndexedCounter},' \
        -e '/{Absinthe.Subscription, Endpoint}/a\\          {InternalTransactionsIndexedCounter, name: InternalTransactionsIndexedCounter},' \
        apps/block_scout_web/lib/block_scout_web/application.ex \
    && awk 'index($0, "defp download_config(_), do: {:error, :invalid_config_url}") { print "  defp download_config(nil), do: local_rate_limit_config()"; print ""; print "  defp local_rate_limit_config do"; print "    :block_scout_web"; print "    |> Application.app_dir(\"priv/rate_limit_config.json\")"; print "    |> File.read()"; print "    |> case do"; print "         {:ok, config} -> decode_config(config)"; print "         error -> error"; print "       end"; print "  end"; next } { print }' apps/block_scout_web/lib/block_scout_web/utility/rate_limit_config_helper.ex > /tmp/rate_limit_config_helper.ex \
    && mv /tmp/rate_limit_config_helper.ex apps/block_scout_web/lib/block_scout_web/utility/rate_limit_config_helper.ex \
    && wget -q -T 30 -t 3 -O /tmp/hex-2.5.1-otp-27.ez \
        https://builds.hex.pm/installs/1.19.0/hex-2.5.1-otp-27.ez \
    && echo "3f327dee07b450ba3d7c8e82c2b75be9d351f1134310558d89f0984133d9b921  /tmp/hex-2.5.1-otp-27.ez" | sha256sum -c - \
    && mix archive.install /tmp/hex-2.5.1-otp-27.ez --force \
    && rm -f /tmp/hex-2.5.1-otp-27.ez \
    && mix local.rebar --force \
    && mix deps.get \
    && mkdir -p /tmp/evision-cache \
    && curl --fail --location --retry 3 --retry-all-errors \
        --output /tmp/evision-cache/evision-nif_2.16-x86_64-linux-musl-0.2.17.tar.gz \
        https://github.com/cocoa-xu/evision/releases/download/v0.2.17/evision-nif_2.16-x86_64-linux-musl-0.2.17.tar.gz \
    && echo "9f08f3ce4cfadff84ac1813800035bcc81ee773587828efcfbbaa050d9639753  /tmp/evision-cache/evision-nif_2.16-x86_64-linux-musl-0.2.17.tar.gz" | sha256sum -c - \
    && mix deps.compile --skip-umbrella-children

FROM ${NODE_BUILDER_IMAGE} AS blockscout-assets

USER root
WORKDIR /src/blockscout
COPY --from=blockscout-deps /app/apps/block_scout_web/assets /src/blockscout/apps/block_scout_web/assets
COPY --from=blockscout-deps /app/deps/phoenix /src/blockscout/deps/phoenix
COPY --from=blockscout-deps /app/deps/phoenix_html /src/blockscout/deps/phoenix_html
RUN chown -R chaimir:chaimir /src/blockscout

USER chaimir
WORKDIR /src/blockscout/apps/block_scout_web/assets
RUN npm ci --ignore-scripts --no-fund \
    && npm run deploy

FROM blockscout-deps AS blockscout-build

COPY --from=blockscout-assets /src/blockscout/apps/block_scout_web/priv/static /app/apps/block_scout_web/priv/static
RUN mix compile \
    && (cd apps/block_scout_web && mix phx.digest) \
    && mkdir -p /opt/release \
    && mix release blockscout \
    && mv _build/${MIX_ENV}/rel/blockscout /opt/release/blockscout

FROM ${BLOCKSCOUT_BUILD_IMAGE}

LABEL org.opencontainers.image.title="tool/blockscout"
LABEL org.opencontainers.image.description="Chaimir Blockscout EVM explorer source-rebuild image"

ARG BLOCKSCOUT_USER=blockscout
ARG BLOCKSCOUT_GROUP=blockscout
ARG BLOCKSCOUT_UID=1000
ARG BLOCKSCOUT_GID=1000
ENV DISABLE_WEBAPP=false
ENV ADMIN_PANEL_ENABLED=false
ENV RELEASE_VERSION=11.2.8
ENV BLOCKSCOUT_VERSION=v11.2.8.+commit.01a56de2

USER root
RUN apk upgrade --no-cache \
    && apk --no-cache --update add jq curl \
    && addgroup --system --gid ${BLOCKSCOUT_GID} ${BLOCKSCOUT_GROUP} \
    && adduser --system --uid ${BLOCKSCOUT_UID} --ingroup ${BLOCKSCOUT_GROUP} --disabled-password ${BLOCKSCOUT_USER}
COPY --from=blockscout-build --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /opt/release/blockscout /app
COPY --from=blockscout-build --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/config_helper.exs /app/config/config_helper.exs
COPY --from=blockscout-build --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/config_helper.exs /app/releases/${RELEASE_VERSION}/config_helper.exs
COPY --from=blockscout-build --chown=${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app/config/assets/precompiles-arbitrum.json /app/config/assets/precompiles-arbitrum.json
RUN mkdir -p /app/dets /app/temp /app/logs /workspace /tmp/blockscout /tmp/tzdata \
    && chown -R ${BLOCKSCOUT_USER}:${BLOCKSCOUT_GROUP} /app /workspace /tmp/blockscout /tmp/tzdata

COPY entrypoint.sh /usr/local/bin/chaimir-blockscout-entrypoint
RUN chmod +x /usr/local/bin/chaimir-blockscout-entrypoint

USER ${BLOCKSCOUT_UID}:${BLOCKSCOUT_GID}
WORKDIR /app
EXPOSE 4000
