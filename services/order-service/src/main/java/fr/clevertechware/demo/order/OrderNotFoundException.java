package fr.clevertechware.demo.order;

class OrderNotFoundException extends RuntimeException {

    OrderNotFoundException(long id) {
        super("Order " + id + " not found");
    }
}
