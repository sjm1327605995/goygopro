#ifndef OCGCORE_H
#define OCGCORE_H

#include <stdint.h>
intptr_t create_duel(int32_t seed);
void start_duel(intptr_t pduel, int32_t options);
void end_duel(intptr_t pduel);
void set_player_info(intptr_t pduel, int32_t playerid, int32_t lp, int32_t startcount, int32_t drawcount);
void get_log_message(intptr_t pduel,unsigned char* buf);
int32_t get_message(intptr_t pduel, unsigned char* buf);
int32_t process(intptr_t pduel);
void new_card(intptr_t pduel, uint32_t code, uint8_t owner, uint8_t playerid, uint8_t location, uint8_t sequence, uint8_t position);
int32_t query_card(intptr_t pduel, uint8_t playerid, uint8_t location, uint8_t sequence, int32_t query_flag,  unsigned char* buf, int32_t use_cache);
int32_t query_field_count(intptr_t pduel, uint8_t playerid, uint8_t location);
int32_t query_field_card(intptr_t pduel, uint8_t playerid, uint8_t location, int32_t query_flag, unsigned char* buf, int32_t use_cache);
int32_t query_field_info(intptr_t pduel, unsigned char* buf);
void set_responsei(intptr_t pduel, int32_t value);
void set_responseb(intptr_t pduel, unsigned char* buf);
int32_t preload_script(intptr_t pduel, const char* script, int32_t len);
typedef struct {
uint32_t code;
uint32_t alias;
uint64_t setcode;
uint32_t type;
uint32_t level;
uint32_t attribute;
uint32_t race;
int32_t attack;
int32_t defense;
uint32_t lscale;
uint32_t rscale;
uint32_t link_marker;
}card_data;
typedef unsigned char*  (*script_reader)(const char*, int*);
typedef uint32_t (*card_reader)(uint32_t, card_data*);
typedef uint32_t (*message_handler)(intptr_t, uint32_t);
extern void set_script_reader(script_reader f);
extern void set_card_reader(card_reader f);
extern void set_message_handler(message_handler f);



// 导出Go函数供C调用
extern unsigned char* goScriptReader(char* data, int *size);
extern void goMessageHandler(intptr_t  pduel, uint32_t size);
extern uint32_t goCardReader(uint32_t card_id,  card_data* data);
#endif  // OCGCORE_H