# Фронтенд: собираем статику Vite и отдаём её nginx.
# nginx же — единственная точка входа: он проксирует /api на бэкенд,
# поэтому фронт и API живут на одном origin и сессионная cookie ходит как своя.
FROM node:22-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
