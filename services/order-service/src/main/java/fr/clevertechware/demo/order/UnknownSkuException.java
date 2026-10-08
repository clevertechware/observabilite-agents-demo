package fr.clevertechware.demo.order;

class UnknownSkuException extends RuntimeException {

    UnknownSkuException(String sku) {
        super("Unknown sku " + sku);
    }
}
