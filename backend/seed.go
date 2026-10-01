package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// seedAdmin crea el primer usuario admin usando ADMIN_EMAIL / ADMIN_PASSWORD si aún no existe.
func seedAdmin() {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		log.Println("ADMIN_EMAIL / ADMIN_PASSWORD no definidos: se omite la creación del admin inicial")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	count, err := adminsCol().CountDocuments(ctx, bson.M{"email": email})
	if err != nil {
		log.Println("No se pudo verificar el usuario admin:", err)
		return
	}
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("No se pudo generar el hash del admin:", err)
		return
	}
	_, err = adminsCol().InsertOne(ctx, AdminUser{
		Email: email, PasswordHash: string(hash), CreatedAt: time.Now(),
	})
	if err != nil {
		log.Println("No se pudo crear el admin inicial:", err)
		return
	}
	log.Println("Usuario admin inicial creado:", email)
}

const demoSeedMarker = "seed_demo_casio_v1"

// seedEnabled decide si corresponde cargar datos demo:
//   - SEED_DEMO_DATA debe ser "true";
//   - APP_ENV=production lo bloquea SIEMPRE, aunque SEED_DEMO_DATA quede en
//     true por error en el .env de producción.
func seedEnabled(appEnv, seedFlag string) bool {
	if strings.ToLower(strings.TrimSpace(seedFlag)) != "true" {
		return false
	}
	return strings.ToLower(strings.TrimSpace(appEnv)) != "production"
}

// seedWatches carga relojes Casio de demostración UNA sola vez por base de
// datos. Después de la primera carga deja una marca en app_meta: aunque el
// Admin archive o borre esos relojes, reiniciar el backend no los vuelve a
// insertar. Nunca borra ni modifica documentos existentes.
//
// Especificaciones tomadas de las fichas oficiales de casio.com (sitio
// internacional). Las imágenes viven en frontend/public/images/relojes/casio.
// Los precios quedan en 0 ("Consultar precio") a propósito: se cargan y
// editan desde el panel Admin, nunca se obtienen de forma automática. Las
// cantidades de stock son de demostración.
func seedWatches() {
	appEnv, flag := os.Getenv("APP_ENV"), os.Getenv("SEED_DEMO_DATA")
	if !seedEnabled(appEnv, flag) {
		if strings.EqualFold(strings.TrimSpace(flag), "true") {
			log.Println("APP_ENV=production: se ignora SEED_DEMO_DATA=true (no se cargan datos demo)")
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Reclamar la marca primero (insert con _id fijo = atómico): si dos
	// instancias arrancan a la vez, solo una carga los datos.
	_, err := metaCol().InsertOne(ctx, bson.M{"_id": demoSeedMarker, "createdAt": time.Now()})
	if mongo.IsDuplicateKeyError(err) {
		return // ya se cargó alguna vez
	}
	if err != nil {
		log.Println("No se pudo registrar la carga demo:", err)
		return
	}

	// Bases creadas antes de existir la marca: si ya hay Casio, no se duplica.
	count, err := watchesCol().CountDocuments(ctx, bson.M{"brandKey": "casio"})
	if err != nil {
		log.Println("No se pudo verificar el catálogo:", err)
		return
	}
	if count > 0 {
		log.Println("Ya hay relojes Casio en el catálogo: se omite la carga demo")
		return
	}

	sample := demoCasioWatches(time.Now())
	docs := make([]interface{}, len(sample))
	for i := range sample {
		docs[i] = sample[i]
	}
	if _, err := watchesCol().InsertMany(ctx, docs); err != nil {
		log.Println("No se pudo cargar el catálogo de ejemplo:", err)
		// Se libera la marca para reintentar en el próximo arranque.
		_, _ = metaCol().DeleteOne(ctx, bson.M{"_id": demoSeedMarker})
		return
	}
	log.Printf("Catálogo Casio de ejemplo cargado (%d relojes)", len(docs))
}

func demoCasioWatches(now time.Time) []Watch {
	list := []Watch{
		{Brand: "Casio", Name: "Casio G-Shock GA-2100-1A1", Model: "GA-2100-1A1", Gender: "hombre",
			Description: "G-Shock analógico-digital de perfil octogonal, heredero del diseño del primer DW-5000C. " +
				"Caja de resina con fibra de carbono que logra un espesor de solo 11,8 mm, en color negro integral.",
			Image: "/images/relojes/casio/ga-2100-1a1.webp",
			Specs: []string{
				"Caja: 48,5 × 45,4 × 11,8 mm",
				"Peso: 51 g",
				"Caja y bisel: carbono / resina",
				"Estructura Carbon Core Guard, resistente a golpes",
				"Resistencia al agua: 200 metros",
				"Cristal mineral",
				"Correa de resina",
				"Hora mundial (31 zonas horarias), cronómetro 1/100 s, temporizador, 5 alarmas",
				"Doble luz LED (esfera y display digital)",
				"Calendario automático completo (hasta 2099)",
				"Pila SR726W × 2, duración aprox. 3 años",
				"Precisión: ±15 segundos por mes",
			},
			StockQuantity: 3, CreatedAt: now, UpdatedAt: now},
		{Brand: "Casio", Name: "Casio Vintage A168WA-1W", Model: "A168WA-1W", Gender: "hombre",
			Description: "Clásico digital de la línea Casio Vintage, con malla metálica ajustable y luz de fondo " +
				"electroluminiscente. Simple y fácil de combinar.",
			Image: "/images/relojes/casio/a168wa-1w.webp",
			Specs: []string{
				"Caja: 38,6 × 36,3 × 9,6 mm",
				"Peso: 50 g",
				"Caja y bisel: resina / cromado",
				"Malla de acero inoxidable con cierre ajustable",
				"Resistente al agua (uso diario)",
				"Cristal de resina",
				"Cronómetro 1/100 s, alarma diaria y señal horaria",
				"Luz de fondo electroluminiscente",
				"Pila CR2016, duración aprox. 7 años",
				"Precisión: ±30 segundos por mes",
			},
			StockQuantity: 2, CreatedAt: now, UpdatedAt: now},
		{Brand: "Casio", Name: "Casio F-91W-1", Model: "F-91W-1", Gender: "hombre",
			Description: "El digital clásico de Casio: liviano, práctico y con luz para leer la hora en la oscuridad.",
			Image:       "/images/relojes/casio/f-91w-1.webp",
			Specs: []string{
				"Caja: 38,2 × 35,2 × 8,5 mm",
				"Peso: 21 g",
				"Caja y correa de resina",
				"Resistente al agua (uso diario)",
				"Cristal de resina",
				"Cronómetro 1/100 s, alarma diaria y señal horaria",
				"Luz LED",
				"Pila CR2016, duración aprox. 7 años",
				"Precisión: ±30 segundos por mes",
			},
			StockQuantity: 1, CreatedAt: now, UpdatedAt: now},
		{Brand: "Casio", Name: "Casio MTP-V002D-1B", Model: "MTP-V002D-1B", Gender: "hombre",
			Description: "Analógico de diseño simple, con esfera negra, indicador de fecha y malla de acero inoxidable.",
			Image:       "/images/relojes/casio/mtp-v002d-1b.webp",
			Specs: []string{
				"Caja: 44 × 37 × 9,6 mm",
				"Peso: 93 g",
				"Malla de acero inoxidable con cierre de triple pliegue",
				"Resistente al agua (uso diario)",
				"Cristal mineral",
				"Analógico de 3 agujas (hora, minuto, segundo)",
				"Indicador de fecha",
				"Pila SR626SW, duración aprox. 3 años",
				"Precisión: ±20 segundos por mes",
			},
			StockQuantity: 2, CreatedAt: now, UpdatedAt: now},
		{Brand: "Casio", Name: "Casio LTP-V002D-7B", Model: "LTP-V002D-7B", Gender: "mujer",
			Description: "Analógico femenino de líneas simples, esfera clara, indicador de fecha y malla de acero inoxidable.",
			Image:       "/images/relojes/casio/ltp-v002d-7b.webp",
			Specs: []string{
				"Caja: 31 × 25 × 9,2 mm",
				"Peso: 53 g",
				"Malla de acero inoxidable con cierre de triple pliegue",
				"Resistente al agua (uso diario)",
				"Cristal mineral",
				"Analógico de 3 agujas (hora, minuto, segundo)",
				"Indicador de fecha",
				"Pila SR626SW, duración aprox. 3 años",
				"Precisión: ±20 segundos por mes",
			},
			StockQuantity: 2, CreatedAt: now, UpdatedAt: now},
		{Brand: "Casio", Name: "Casio Vintage LA670WA-1", Model: "LA670WA-1", Gender: "mujer",
			Description: "Digital de estilo retro de la línea Casio Vintage, en tamaño compacto, con alarma, " +
				"temporizador y malla metálica ajustable.",
			Image: "/images/relojes/casio/la670wa-1.webp",
			Specs: []string{
				"Caja: 30,3 × 24,6 × 7,3 mm",
				"Peso: 26 g",
				"Caja y bisel: resina / cromado",
				"Malla de acero inoxidable con cierre ajustable",
				"Resistente al agua (uso diario)",
				"Cristal de resina",
				"Cronómetro 1/10 s y temporizador de hasta 30 minutos",
				"Alarma diaria y señal horaria",
				"Pila CR1216, duración aprox. 2 años",
				"Precisión: ±30 segundos por mes",
			},
			StockQuantity: 0, CreatedAt: now, UpdatedAt: now},
	}
	for i := range list {
		list[i].BrandKey = brandKeyFor(list[i].Brand)
		list[i].ModelKey = modelKeyFor(list[i].Model)
	}
	return list
}
