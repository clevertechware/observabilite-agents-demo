package fr.clevertechware.demo.order;

import jakarta.validation.Valid;
import java.net.URI;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
class OrderController {

    private final OrderService orders;

    OrderController(OrderService orders) {
        this.orders = orders;
    }

    @PostMapping("/orders")
    ResponseEntity<Order> create(@Valid @RequestBody OrderRequest request) {
        Order order = orders.place(request);
        return ResponseEntity.created(URI.create("/orders/" + order.id())).body(order);
    }

    @GetMapping("/orders/{id}")
    Order get(@PathVariable long id) {
        return orders.find(id);
    }

    @GetMapping("/healthz")
    String healthz() {
        return "ok";
    }
}
