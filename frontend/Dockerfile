# --- Etapa de build ---
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
# Instalación reproducible desde package-lock.json.
RUN npm ci
COPY . .
# La URL de la API se fija al momento del build (Vite la incrusta en el bundle).
ARG VITE_API_URL=http://localhost:8080/api
ENV VITE_API_URL=$VITE_API_URL
# Datos de contacto públicos (vacíos = el sitio oculta esos botones).
ARG VITE_WHATSAPP_NUMBER=
ARG VITE_CONTACT_EMAIL=
ARG VITE_INSTAGRAM_URL=
ARG VITE_FACEBOOK_URL=
ENV VITE_WHATSAPP_NUMBER=$VITE_WHATSAPP_NUMBER
ENV VITE_CONTACT_EMAIL=$VITE_CONTACT_EMAIL
ENV VITE_INSTAGRAM_URL=$VITE_INSTAGRAM_URL
ENV VITE_FACEBOOK_URL=$VITE_FACEBOOK_URL
RUN npm run build

# --- Imagen final: nginx sirviendo los archivos estáticos ---
FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
