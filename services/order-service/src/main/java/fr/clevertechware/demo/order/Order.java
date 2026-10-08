package fr.clevertechware.demo.order;

import java.math.BigDecimal;

record Order(long id, String sku, int quantity, BigDecimal unitPrice, BigDecimal total) {
}
