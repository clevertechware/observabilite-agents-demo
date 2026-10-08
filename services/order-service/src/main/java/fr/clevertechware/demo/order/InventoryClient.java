package fr.clevertechware.demo.order;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;
import org.springframework.web.client.RestClientException;

@Component
class InventoryClient {

    private final RestClient restClient;

    InventoryClient(RestClient.Builder builder, @Value("${inventory.base-url}") String baseUrl) {
        this.restClient = builder.baseUrl(baseUrl).build();
    }

    Reservation reserve(String sku, int quantity) {
        try {
            Reservation reservation = restClient.post()
                    .uri("/reservations")
                    .body(new ReservationRequest(sku, quantity))
                    .retrieve()
                    .onStatus(status -> status.isSameCodeAs(HttpStatus.CONFLICT), (request, response) -> {
                        throw new InsufficientStockException(sku);
                    })
                    .onStatus(status -> status.isSameCodeAs(HttpStatus.NOT_FOUND), (request, response) -> {
                        throw new UnknownSkuException(sku);
                    })
                    .body(Reservation.class);
            if (reservation == null || reservation.unitPrice() == null || reservation.total() == null) {
                throw new InventoryUnavailableException("Inventory returned an incomplete reservation");
            }
            return reservation;
        } catch (RestClientException e) {
            throw new InventoryUnavailableException("Inventory call failed: " + e.getMessage(), e);
        }
    }
}
