package fr.clevertechware.demo.order;

import java.math.BigDecimal;

record Reservation(String sku, int quantity, BigDecimal unitPrice, BigDecimal total) {
}
