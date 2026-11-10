# nukri.urdadx.com in one image: the landing page (apps/web, an Astro server),
# the docs (apps/docs, a static Blume build) at /docs, and the install script at
# /install.sh, all served by the landing page's Node server.
#
# The build context is the repository root, for install.sh:
#   docker build -f www/Dockerfile -t nukri-www .
#   docker run -p 4321:4321 nukri-www

FROM node:24-slim AS base
RUN npm install --global pnpm@11.23.0
WORKDIR /www

FROM base AS build
# Dependencies first, so they stay cached until a package.json or the lockfile
# changes. The root postinstall reads apps/web/.env.schema.
COPY www/package.json www/pnpm-lock.yaml www/pnpm-workspace.yaml ./
COPY www/apps/web/package.json www/apps/web/.env.schema apps/web/
COPY www/apps/docs/package.json apps/docs/
COPY www/packages/config/package.json packages/config/
# Hoisted: Astro's server bundle imports Astro's own dependencies by name, which
# pnpm's default layout keeps out of apps/web/node_modules.
RUN pnpm install --frozen-lockfile --config.node-linker=hoisted

COPY www/ ./
RUN pnpm --filter docs build
RUN NODE_ENV=production pnpm --filter web build
# The landing page's production dependencies alone, for the runtime image.
RUN pnpm --filter web deploy --prod --legacy --config.node-linker=hoisted /deploy

FROM node:24-slim
ENV NODE_ENV=production HOST=0.0.0.0 PORT=4321
WORKDIR /app
COPY --from=build /deploy/node_modules node_modules
# varlock loads the env schema from the working directory when the server starts.
COPY --from=build /deploy/package.json /www/apps/web/.env.schema ./
COPY --from=build /www/apps/web/dist dist
# Blume links every page and asset under /docs (deployment.base), so its build
# goes in that folder of the files the server serves as they are.
COPY --from=build /www/apps/docs/dist dist/client/docs
COPY install.sh dist/client/install.sh
USER node
EXPOSE 4321
CMD ["node", "dist/server/entry.mjs"]
