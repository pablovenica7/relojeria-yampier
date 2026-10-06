package database

import (
	"time"

	"relojeria-yampier/internal/domain"
)

// DemoCasioWatches son los relojes de demostración (especificaciones de
// casio.com; precio 0 = a consultar; cantidades de stock de demostración).
func DemoCasioWatches(now time.Time) []domain.Watch {
	list := []domain.Watch{
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
		list[i].BrandKey = domain.BrandKeyFor(list[i].Brand)
		list[i].ModelKey = domain.ModelKeyFor(list[i].Model)
	}
	return list
}
