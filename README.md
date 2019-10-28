# Write a Torrent Client in Go

<!--
TEST CHNAGE`
- [Introduction](#Introduction)
- [Theory](#Theory)
    - [Reading the torrent file](#Reading_the_torrent_file)
    - [Pieces and Blocks, Data](#Pieces_and_Blocks)
    - [Getting the Peers via Tracker](#Contents)
    - [Downloading Data from the Peers](#Contents)
    - [Constructing Files from Pieces, Single & Multi](#Contents)
- [Code](#Code)
-->

<!-- 
<h1 style="text-align:center;">Theory</h1> -->

**BitTorrent** is a **peer-to-peer** (P2P) file sharing protocol over the Internet, where a computer can join a swarm of other computers to exchange **pieces of data** between each other. Each computer that is connected to the network is called a **Peer**. And, each **peer** is connected to **multiple peers at the same time**, and thus downloading or uploading to multiple peers at the same time. Unlike normal downloads, a torrent download doesn't put any signoficant load on a single peer's bandwidth, so it's faster.

Data sharing over BitTorrent starts from a `.torrent` file. The file contains information about where to look for other peers, and how to assemble the file from the pieces (the file/files we need). A **BitTorrent-Client** is software that communicates to all the peers and manages downloads or uploads between between them.

One of the things that the `.torrent` file contains is, an address of a **tracker**. Think of it as a server that **keeps track of all the clients** that has the file you need, aka peers. A client can get information about other peers by sending something called a **tracker-request** to the tracker. Specificly, it gets each peer's **ip-address** and **port** from the tracker-response. And, once the client gets the peer addresses, it can download piece of data from them.

<img src="https://gitlab.com/ritwik310/blog-documents/raw/master/Write-a-Torrent-Client-in-Go-0/Torrent-Client-0.png" />

In this tutorial we are gonna write a **BitTorrent-Client** from scratch in **Go**. Our focus will be downloading files from the peers. We are not gonna focus on file sharing for now. By the end, you can download files via the BitTorrent-protocol from the command-line.

# Reading the torrent file

To download stuff via **BitTorrent** you need a **torrent-file** first. The file contains information about the data that you wanna download, and where to find the tracker. For this tutorial we are gonna be using **Ubuntu 19.04 Server (64-bit)** torrent downloader. Although it's a large file its good for development purposes, mainly because it's a **single file** download and we can easily find of active **seeders** (peers that have the data you want). You can download it by clicking [this](http://releases.ubuntu.com/19.04/ubuntu-19.04-live-server-amd64.iso.torrent), or by going here [ubuntu.com/download/alternative-downloads](https://ubuntu.com/download/alternative-downloads)

The `.torrent` file contains **Bencode** encoded metadata for the torrent. **Bencode** is data serialization format. You can read more about it here - [wikipedia.org/wiki/Bencode](https://en.wikipedia.org/wiki/Bencode)

<!-- To decode **bencoded dictionaries** we are gonna be using a 3ed party go package - [github.com/marksamman/bencode](https://github.com/marksamman/bencode) -->

If we decode the `ubuntu-19.04-live-server-amd64.iso.torrent` file that we have downloaded earlier, it's gonna look something like this,

```json
{
   "announce": "http://torrent.ubuntu.com:6969/announce",
   "announce-list": [
      [
         "http://torrent.ubuntu.com:6969/announce"
      ],
      [
         "http://ipv6.torrent.ubuntu.com:6969/announce"
      ]
   ],
   "comment": "Ubuntu CD releases.ubuntu.com",
   "creation date": 1555564384,
   "info": {
      "length": 784334848,
      "name": "ubuntu-19.04-live-server-amd64.iso",
      "piece length": 524288,
      "pieces": "<hex>E2 0F 7E ...</hex>"
   }
}
```

> To decode the bencoded dictionary and represent it in JSON, I used - [chocobo1.github.io/bencode_online](https://chocobo1.github.io/bencode_online)

If we look at the fields of decoded dictionary above, they are just **key-value pairs** describing some information about the torrent. Let's break down the contents of the `.torrent` file of a typical **single-file torrent downloader**, like the one we just downloaded.

- `announce` - A **URL** string. It is called the **announce-url of tracker**. If we send a specific type of request to that URL, it will respond with data about other peers.

- `announce-list` - **[Optional]** - This is a **list of lists of URLs**, and will contain a list of tiers of announces. If a client is compatible with the **multitracker specification**, it will ignore the **announce** property and only use the URLs in **announce-list**, if it's present. 

- `info` - A **dictionary** that describes the **file** that you want to download. In our **single file torrent** example the info dictionary contains the following fields,

   - `length` - An integer corresponding to **length** of the **file** that we want to download (single-file) in bytes.
   - `name` - **Name** of the file.
   - `piece length` - **Length** of each **piece** in bytes. More about the pieces [here](#s)
   - `pieces` - A string consisting of the **concatenation of 20-byte SHA1 hash values** for all pieces.

   - `md5sum` - **[Optional]** - A 32-character **hexadecimal string** corresponding to the **MD5 sum** of the **file**.

For **multi-file torrents** the contents of `info` dictionary is a little different though.

- `info` - ...

   - `piece length` - Just like single-file, an integer representing the **length** of each **piece** in bytes.
   - `pieces` - A string consisting of the **concatenation of 20-byte SHA1 hash values** for all pieces. Just like in the single-file.

   - `name` - The **name of the directory** in which to store all the files.
   - `files` - A **list of dictionaries**, one for each file. **Each dictionary** in this list contains the following fields.

      - `length` - **Length** of one file in bytes.
      - `md5sum` - **MD5 sum** of that file.
      - `path` - A **list** containing one or more string elements that together represent the **path and filename**.

There are some more optional fields in the bencoded dictionary. You can read about those here - [wiki.theory.org/index.php/BitTorrentSpecification](https://wiki.theory.org/index.php/BitTorrentSpecification#Metainfo_File_Structure).

> NOTE: To better understand **pieces**, go through [this](#this).

# Getting Peers from Tracker

As mentioned earlier, **to find all the other peers** a client needs to send a request to something called a **tracker**. This is called a **announce-request** or **tracker-request**. The address of the tracker can be found in the `announce` property of the `.torrent` file.

In the `.torrent` file that we just downloaded, the **announce URL** happens to be `http://torrent.ubuntu.com:6969/announce`. You can say that the tracker uses **HTTP (TCP)** for communication from the `http://` at the start of the URL. But that's not necessary. Some trackers communicate over **UDP** too, which is also an internet protocol, a little different from **TCP** though (TCP is what HTTP is build over). More about **UDP** [here](#here).

To get all the other peers, a client needs to send a **HTTP GET-request** to the **announce URL** with some unique identifier of the torrent & client in the URL-params (in the case of HTTP URLs). The implimentation of tracker-request over **UDP** is a little different though.

The required parameters for the request are the following...

- **info_hash:** The **20 byte SHA1 hash** of the **bencode encoded** form of the `info` value from the `./torrent` file. Code for extracting **info_hash** is [here](#here).
- **peer_id:** A **20 byte** long unique string, to be used as the **id** for the download. A client needs to generate its own id at random at the start of a new download. There are certain rules to follow while generating a client's **peer_id** what we will discuss [later](#later). The code for generating peer_id is [here](#here).
- **ip:** IP-address of the client machine. This is an optional parameter though, cause the tracker can always feigure out the IP-address from the request itself.
- **port:** The port that the client is listening on for other peers to connect. The ports used for BitTorrent are typically 6881-6889.
- **uploaded:** The total amount uploaded so far, encoded in base 10 ASCCI.
<!--In the case of the client that we are building it's gonna be 0 in value while tracker request. -->
- **downloaded:** The total amount downloaded so far, encoded in base 10 ASCCI.
- **left:**: The number of bytes this peer still has to download, encoded in base 10 ASCCI.
- **event:** Indicates one of these events - started, completed and stopped. For UDP requests there's pre-assigned numbers for the events, but more on that [later](#s)
- **numwant:** Maximum number of peers that a client wants the tracker send back in response.

There are some more optional parameters too. To learn more - [wiki.theory.org/index.php/BitTorrentSpecification](https://wiki.theory.org/index.php/BitTorrentSpecification#Tracker_Request_Parameters)

For **TCP/HTTP tracker-request** the data needs to be put as **URL-encoded** key-value pairs in **query string**. Once the **tracker** get's the request it writes back details about other peers **on that connection**.

For the trackers that uses **UDP** it's a little different though. First, a client needs to send a **connection request** to the tracker-url with some specific data and wait for the tracker to respond back with something called an **connection ID**. Then, the client again needs to send another request with the **data mentioned above** and the **connection ID** encoded in the packet. And finally, the tracker writes back the **data about other peers** to the client. In the case of UDP we need to take care of requesting data multiple times in different intervals to overcome network failures.

The implementation/details for both **TCP/HTTP** and **UDP** is [here](#here).

<!--http://www.bittorrent.org/beps/bep_0015.html-->

> UDP: UDP (User Datagram Protocol), something something something. Small packets. Faster. No connection establishment. Network failure.

<!--Code [here](#code)-->

<!--For example, the tracker request for the Ubuntu-Server-Torrent is gonna look something like, `http://torrent.ubuntu.com:6969/announce?event=started&downloaded=0&uploaded=0&port=6881&left=784334848&ip=XX.XX.XX.XXX&numwant=20&peer_id=-TC0001-somerandom20&info_hash=%B7%B0%FB%ABt%A8%5DJ%C1pf%2CdY%82%A8b%82dU`-->

# Pieces of Data

Once a client gets the **peers**, a client needs to request data from them. In BitTorrent, the whole data (full file/files) is divided into **multiple pieces**. The `piece length` property from `./torrent` file (within `info` dictionary) represents the **length (size) of each piece** in bytes (equal for all pieces). For example, in our `ubuntu-19.04-live-server-amd64.iso.torrent` file, each piece length is **524288 bytes**, and it has **784334848 / 524288 = 1496 pieces**.

<!--We can also extract the **SHA1 hash of each piece** from the `pieces` field, which is going to be useful later.-->

The pieces are **uniquely identified among the peers by it's index**. Before downloading, the peers will let us know which pieces they have and we can request only those pieces from that peer. To make the download efficient, we need to request different pieces from different peers. And, that's how torrent download is fatser than normal server-client file downloads.

While downloading pieces from the **peers**, we request pieces in **chunks**. Typically, each chunk happens to be **2^14 (16384) bytes in size**. This is called a **block**.

**NOTE:** Though the length of all pieces is equal, **the last piece** might not be full. For example, Let's say the file is total **584600 bytes** in size (approximately 571 kb). And let's also assume that each piece is going to be **86920 bytes** in long (from the `piece length`). In that case, there will be **6 pieces**. Each piece's size will be **86920 bytes**. And the **last piece** will contain **584600 % 86920 = 63080 bytes** of useful data.

<img src="https://gitlab.com/ritwik310/blog-documents/raw/master/Write-a-Torrent-Client-in-Go-0/Torrent-Client-Pieces-1.png" />

Just like pieces, every block is **16384 bytes** long but for the **last one**. In the case of blocks though, the length of the last block doesn't stay the same. Details [here](#here). In the example above, **16384 bytes** for each block gives us **6 blocks for a piece**. And, the **last block's length** becomes, **86920 % 16384 = 5000 bytes**.

<img src="https://gitlab.com/ritwik310/blog-documents/raw/master/Write-a-Torrent-Client-in-Go-0/Torrent-Client-Blocks-2.png" />

> **NOTE:** The original BitTorrent specification uses the term **"piece"** when describing the peer protocol, but as a different term than **"piece"** in the `./torrent` file. For that reason, the term **"block"** will be used in this tutorial to describe the little chunks of data that is exchanged between two peers.


# Peer to Peer Communication

Once it gets the **IP-address** and **Ports** of other **peers** from tracker-request, a client needs to **download pieces** of data from them. Also, share pieces with other peers. This whole communication between **peers** typically operates over **TCP**, and clients need to follow a **certain protocol (bittorrent peer protocol)**. Simply, one client establishes a **TCP** connection with another client (peer), and sends different **messages** back and forth to communicate and share data between each other.

Before we go any further talking about different messages, there are some **terms** that we need to be clear about. These typically indicates the **state of a peer-connection**.

- **Choked and Unchoked:** Choked and Unchoked represents **state of a remote peer**. If a peer has **choked** a client, that means the client is **not allowed to request pieces of data** from that peer. And, **unchoked** means that the client **can request data** from that peer. After connecting to a new **peer**, the state (of that peer connection) is **choked by default**. When we recieve a **unchoke-message** from that peer, the state changes to **unchoked**. More on that [here](#here)

- **Interested:** Whether or not the remote peer is **interested** in something this client has to offer. This is a notification that the remote peer will begin **requesting blocks** (after the client unchokes them). In this tutorial we are not gonna foucs on sharing data though.

### Handshake

Once a **TCP** connection is established with a **peer**, a client can start sending and recieving message from it. The communication between the client and the remote **peer** starts by sending a specific type of message that contains an **unique identifier** of the torrent and the client (a peer-ID). And the peer also **write's back** a similar kind of message on that connection. This is called a **Handshake**, and the message is called a **handshake-message**. All the data sharing, request and other messeges are sent after the **handshake**.

The handshake-message consists of multiple values concatenated together is a specific order...

>**PSTR-Len + PSTR + ReservedBytes + InfoHash + PeerID**

>**PSTR-Len** - **Int8** - Length of **Protocol-String** (1 byte long)   
**PSTR** - **Int32** - String identifier of the **peer-protocol**, more about that just below (variable length)   
**ReservedBytes** - 8 reserved bytes. All current implementations use all zeroes (8 bytes long)   
**InfoHash** - 20-byte **SHA1 hash** of the **bencode encoded** form of the `info` dictionary (20 bytes long)   
**PeerID** - 20-byte string used as a **unique ID** for the client. Same as tracker-request (20 bytes long)   

**Peer-Protocol String (PSTR):** In version 1.0 of the BitTorrent protocol, **PSTR-Len = 19**, and **PSTR = "BitTorrent protocol"**.

The code for constructing the handshake message [here](#here)

### Messages

Typically message exchange between two peers happens after the **handshake**. A message is nothing but a **stream of data** sent from a peer to another, following a certain protocol. All the messages other than handshake-message, has very simillar structure. Every message has three parts - **Length**, **ID**, **Payload**. The structure of a message looks something like the following...

> **Length + ID + Payload**

> **Length** - **Int32** - Length of the rest of the message data (4 bytes long)   
**ID** - **Int8** - Represents what message is it (1 byte long)   
**Payload** - varies for different messages (Optional)   

Here's a list of messages that are used in BitTorrent-Peer-Protocol...

1. **Choke:** Choke message is recieved when a peer doesn't wanna share data with a client. This indicates a peer-connection to be switching to Choked state. <br/>**Len=0001 + ID=0**

2. **Unchoke:** Unchoke message is recieved when a peer wants to share data with the client. Switches the peer-connection's state to Unchoked. <br/> **Len=0001 + ID=1**

3. **Interested:** Interested message is sent to a peer, when a client is interested in downloading some data the peer has to offer. <br/> **Len=0001 + ID=2**

4. **Not-interested:** Just opposite of interested-message. Sent when a client is not interested in downloading any data that the peer has to offer. <br/> **Len=0001 + ID=3**

5. **Have:** This message lets a client know what pieces a peer has. The **payload** of the message is a **Int32** specifying the **zero-based piece index**. Recieving this message from a peer means that the **peer has the piece** and the recieving client can download it. As mentioned earlier, pieces uniquely identified among the peers by it's index. <br/>**Len=0005 + ID=4 + Piece-Index**

6. **Bitfield:** The Bitfield message is another way to let a client know what pieces a peer has to offer, but all at once. It does it by sending a **string of bits**, one for each piece of the whole data. The index of the **each bit** represents the **piece-index**. If **value of a bit is 1 then the piece is present** (piece at that index), else **if 0 then the piece is absent**. Code for decoding the Bitfield message is [here](#here) <br/>**Len=(0001+X) + ID=5 + Bitfield** (where, X = Length of the Bitfield)

7. **Request:** Request-message is used to request pieces of data from a peer. To be exact the request-message requests a chunk of a piece, A.K.A. a **block**. The payload for request-message contains the **piece-index (Int32)**, the **offset where the block/chunk starts (Int32)** and **length of the requested block (Int32)**. Code for constructing request-message is [here](#here) <br/> **Len=0013 + ID=6 + Piece-Index + Begin + Length**

8. **Piece:** Piece-message is used to share pieces of data among peers. The message contains a piece of actual data that has been requested from a peer. The payload conststs of **piece-index (Int32)**, **byte offset of the block within the piece**, and the **block of data**. Code for reading from piece-message is [here](#here) <br/> **Len=(0009+X) + ID=7 + Piece-Index + Begin + Block** (where, X = Length of the Block)

9. **Cancel:** Cancel-message used to cancel block requests. **Len=0013 + ID=8 + Piece-Index + Begin + Length**

10. **Keep-alive:** Peers may close a connection if they receive no messages for a certain period of time. So a keep-alive message must be sent to maintain the connection alive if no command have been sent for a given amount of time (generally 2 minutes). Unlike other messages, there's no payload or ID for this message, it's just a Int32 specifying the length (which is 0). <br/> **Len=0000**

### Message Flow

Typically messaging between a client that wants to download data and a remote peer goes something like this...

1\. The client sends a handshake message to a peer   
2\. The client recieves a handshake message from that peer   
3\. The client recieves multipe have-messages or a bitfield-message, or both   
4\. The client recieves an unchoke message from the peer   
5\. The client starts requesting pieces one by one from the peer, by sending request-messages (goes on for multiple pieces)   
6\. The client waits for piece messages from the peer and saves the pieces of data (goes on for multiple pieces)   

<img src="https://gitlab.com/ritwik310/blog-documents/raw/master/Write-a-Torrent-Client-in-Go-0/Torrent-Client-P2P-Messaging-3.png" />

Messaging usually takes place with multiple peers at the same time, to make the download efficient. The client needs to feigure out what to request from which peer. The client need to take factors like what pieces a peer has and even distribution of requests among peers into consideration. The algorithm for requesting pieces from peers is [here](#here)

# Constructing Files from Pieces

### Single-File
### Multi-File


# More to be added...
=======
# Torrent Client
