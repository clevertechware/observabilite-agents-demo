package fr.clevertechware.demo.order;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

@Service
class OrderService {

    private static final Logger log = LoggerFactory.getLogger(OrderService.class);

    private final InventoryClient inventory;
    private final OrderRepository repository;

    OrderService(InventoryClient inventory, OrderRepository repository) {
        this.inventory = inventory;
        this.repository = repository;
    }

    Order place(OrderRequest request) {
        Reservation reservation = inventory.reserve(request.sku(), request.quantity());
        Order order = repository.insert(
                request.sku(),
                request.quantity(),
                reservation.unitPrice(),
                reservation.total());
        log.info("Order {} placed for sku {} (quantity {})", order.id(), order.sku(), order.quantity());
        return order;
    }

    Order find(long id) {
        return repository.findById(id).orElseThrow(() -> new OrderNotFoundException(id));
    }
}
