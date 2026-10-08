package fr.clevertechware.demo.order;

class InsufficientStockException extends RuntimeException {

    InsufficientStockException(String sku) {
        super("Insufficient stock for sku " + sku);
    }
}
