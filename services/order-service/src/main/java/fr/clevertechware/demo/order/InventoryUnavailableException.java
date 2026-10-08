package fr.clevertechware.demo.order;

class InventoryUnavailableException extends RuntimeException {

    InventoryUnavailableException(String message, Throwable cause) {
        super(message, cause);
    }

    InventoryUnavailableException(String message) {
        super(message);
    }
}
