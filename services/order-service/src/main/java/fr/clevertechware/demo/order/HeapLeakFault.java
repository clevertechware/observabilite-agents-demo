package fr.clevertechware.demo.order;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

@Component
class HeapLeakFault {

    private final int bytesPerRequest;
    private final List<byte[]> retained = Collections.synchronizedList(new ArrayList<>());

    HeapLeakFault(@Value("${FAULT_HEAP_LEAK_BYTES_PER_REQUEST:0}") int bytesPerRequest) {
        this.bytesPerRequest = bytesPerRequest;
    }

    void onRequest() {
        if (bytesPerRequest > 0) {
            retained.add(new byte[bytesPerRequest]);
        }
    }

    long retainedBytes() {
        return (long) retained.size() * bytesPerRequest;
    }
}
