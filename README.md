# booking-service

booking-service: reservas, gestión de franjas horarias, reviews conductor/propietario

GET /parking-spots/{id}/availability: Consultar el calendario de franjas horarias libres y ocupadas de una cochera.

GET /parking-spots/{id}/bookings: Listar las reservas recibidas por una cochera específica.

POST /bookings: Crear una intención de reserva para una cochera en un slot horario específico (inicia el estado PENDING_PAYMENT).

GET /bookings/{id}: Obtener el detalle y estado actual de una reserva.

POST /bookings/{id}/cancel: Cancelar una reserva.

GET /users/{id}/bookings/history: Listar el histórico de reservas del usuario.

POST /bookings/{id}/reviews: Registrar la calificación con estrellas y comentarios al finalizar la estadía (tanto para conductor como para propietario).

GET /parking-spots/{id}/reviews: Listar las calificaciones recibidas por una cochera (para mostrar rating y comentarios en el detalle que ve el conductor).
