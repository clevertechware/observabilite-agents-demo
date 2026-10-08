package fr.clevertechware.demo.order;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Positive;

record OrderRequest(@NotBlank String sku, @Positive int quantity) {
}
