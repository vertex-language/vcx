// §11.8.3 [class.access.base]/1 -- a private member is not accessible outside
// its class and its friends.
class Box {
    int shut = 1;
};
int f(Box& b) { return b.shut; }
